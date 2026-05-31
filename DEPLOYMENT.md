# Deployment Guide — System Design Projects on Oracle Cloud Free Tier

All 50 projects run on a single **Oracle Cloud Infrastructure (OCI) free-tier ARM64 VM**
(4 OCPUs, 24 GB RAM — the Ampere A1 shape). Cost: $0/month, no expiry.

---

## Architecture Overview

```
Internet
   │
   │  HTTPS (port 443) — when domain is added
   │  HTTP  (port 80)  — current mode
   ▼
┌─────────────────────────────────────────────────────┐
│  OCI VM  (Ubuntu 22.04, ARM64 / aarch64)           │
│                                                     │
│  Caddy  (reverse proxy, TLS, path routing)          │
│    /p01/ → :8080   /p02/ → :8081   ...              │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  infra/  (Docker Compose — always running)  │   │
│  │    postgres:5432  (shared across projects)  │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ┌──────────────┐  ┌──────────────┐                │
│  │  p01 :8080   │  │  p02 :8081   │  ...           │
│  │  rate-limiter│  │  url-shorter │                │
│  └──────────────┘  └──────────────┘                │
└─────────────────────────────────────────────────────┘
```

Each project is a self-contained Docker Compose stack. All projects share one
PostgreSQL container (provisioned via `infra/`). No Redis — projects use in-process
caching where needed.

See [infra/ports.md](infra/ports.md) for the full port assignment table.

---

## Step 1 — OCI Console: Open Ports 80 and 443

OCI adds a network-level security list on top of the VM firewall. You must open
ports 80 and 443 in the OCI Console **even if ufw is already configured**.

1. Log in to [cloud.oracle.com](https://cloud.oracle.com)
2. Go to **Networking → Virtual Cloud Networks**
3. Click your VCN → **Security Lists** → **Default Security List**
4. Click **Add Ingress Rules** and add **two rules**:

| Source CIDR | Protocol | Destination Port |
|---|---|---|
| `0.0.0.0/0` | TCP | `80` |
| `0.0.0.0/0` | TCP | `443` |

5. Click **Add Ingress Rules** to save.

> If you don't see a VCN, your VM may be using an instance-level firewall.
> In that case go to: Compute → Instances → your instance → Attached VNICs →
> Subnet → Security List and repeat the same steps.

---

## Step 2 — SSH into the VM

```bash
# Replace with your actual VM public IP and key path
ssh -i ~/.ssh/oci_key ubuntu@<VM_PUBLIC_IP>
```

OCI Ubuntu images default to the `ubuntu` user. If you used a different image
the username may be `opc` (Oracle Linux) or `admin`.

---

## Step 3 — Bootstrap the VM (one-time)

Clone the repo and run the setup script:

```bash
git clone https://github.com/ankitsriv89/system-design.git
cd system-design
bash infra/setup.sh
```

The script will:
- Install Docker, Docker Compose plugin, Caddy
- Configure ufw firewall (SSH + 80 + 443)
- Start the shared postgres container
- Install and start Caddy with the reverse proxy config
- Print next steps

> **After the script completes**, if Docker was just installed, **log out and back in**
> so the `docker` group membership takes effect before running deploy commands.

---

## Step 4 — Deploy Your First Project

```bash
cd ~/system-design

# Deploy project 02 (URL Shortener)
./infra/deploy.sh 02-url-shortener
```

Then visit: `http://<VM_PUBLIC_IP>/p02/`

---

## Deploying Updates

After you push new code from your local machine to GitHub, SSH into the VM and run:

```bash
cd ~/system-design
./infra/deploy.sh 02-url-shortener
```

The deploy script:
1. `git pull` — fetches latest code
2. `docker compose build --pull` — rebuilds the image (uses layer cache; only rebuilds changed layers)
3. `docker compose up -d` — recreates only changed containers (unchanged ones keep running)

Zero-downtime for stateless projects: Docker starts the new container before stopping the old one.

---

## Deploying All Projects at Once

```bash
./infra/deploy.sh all
```

This deploys every project directory that has a `docker-compose.yml`. Projects without
one are skipped.

---

## Environment Variables / Secrets

For projects that need secrets (API keys, passwords etc.), create a `.env` file
**on the VM** alongside the project's `docker-compose.yml`. The `.env` file is
gitignored and never committed.

```bash
# On the VM:
cat > ~/system-design/projects/02-url-shortener/.env << 'EOF'
DATABASE_URL=postgres://url:url@postgres:5432/urlshortener?sslmode=disable
BASE_URL=http://<VM_PUBLIC_IP>/p02
EOF
```

Docker Compose automatically reads `.env` files from the same directory as `docker-compose.yml`.

---

## Adding a Domain (Later)

When you have a domain pointing at the VM's public IP:

1. Create a DNS A record: `*.yourdomain.com → <VM_PUBLIC_IP>`
2. Edit `/etc/caddy/Caddyfile` on the VM:
   - Uncomment the HTTPS subdomain blocks at the bottom of the file
   - Delete (or comment out) the `:80 { ... }` block
   - Replace `yourdomain.com` with your actual domain
3. Reload Caddy:
   ```bash
   sudo systemctl reload caddy
   ```
   Caddy automatically fetches Let's Encrypt TLS certificates. No certbot needed.

The `infra/caddy/Caddyfile` in the repo has pre-written HTTPS blocks ready to uncomment.

---

## Adding a New Project

1. Build the project in `projects/NN-project-name/` with a `docker-compose.yml`
2. Assign it the correct port from [infra/ports.md](infra/ports.md)
3. Add a `handle_path /pNN/*` block to `infra/caddy/Caddyfile` in the repo
4. Commit and push
5. On the VM:
   ```bash
   ./infra/deploy.sh NN-project-name
   sudo cp infra/caddy/Caddyfile /etc/caddy/Caddyfile
   sudo systemctl reload caddy
   ```

---

## Useful Commands on the VM

```bash
# See all running containers
docker ps

# Logs for a specific project
docker compose -f ~/system-design/projects/02-url-shortener/docker-compose.yml logs -f

# Restart a single project
docker compose -f ~/system-design/projects/02-url-shortener/docker-compose.yml restart

# Stop a project (free its RAM)
docker compose -f ~/system-design/projects/02-url-shortener/docker-compose.yml down

# Check postgres is healthy
docker exec $(docker ps -qf name=infra-postgres) pg_isready -U admin

# Connect to postgres
docker exec -it $(docker ps -qf name=infra-postgres) psql -U admin

# Caddy status / logs
sudo systemctl status caddy
sudo journalctl -u caddy -f

# Disk / memory
df -h
free -h

# Prune unused Docker images (free disk space)
docker image prune -f
```

---

## Shared Postgres — Adding a Database for a New Project

The shared postgres init script runs only on first start. For projects built after
the first `docker compose up`, create the DB manually:

```bash
docker exec -it $(docker ps -qf name=infra-postgres) psql -U admin -c \
  "CREATE USER myuser WITH PASSWORD 'mypassword';"

docker exec -it $(docker ps -qf name=infra-postgres) psql -U admin -c \
  "CREATE DATABASE myproject OWNER myuser;"
```

Or add a new `infra/initdb/NN_projectname.sql` file and recreate the postgres
container (data is preserved in the `postgres_data` volume):

```bash
docker compose -f infra/docker-compose.yml up -d --force-recreate postgres
```

---

## OCI Free Tier Limits

| Resource | Free Tier |
|---|---|
| VM shape | VM.Standard.A1.Flex (ARM64) |
| OCPUs | 4 (shared across all VMs) |
| RAM | 24 GB |
| Storage | 200 GB block volume |
| Network | 10 TB outbound / month |
| Duration | Lifetime (no expiry) |

The 4 OCPU / 24 GB RAM shape comfortably runs 50 small containers + postgres + Caddy.

---

## Troubleshooting

**Container won't start — postgres not ready**

The project's `docker-compose.yml` depends on the `infra` external network. Make sure
infra is running first:
```bash
docker compose -f ~/system-design/infra/docker-compose.yml up -d
```

**`permission denied` running docker**

Log out and back in after the setup script installs Docker. The `docker` group
membership only takes effect on a new login session.

**Port already in use**

Check [infra/ports.md](infra/ports.md) and ensure no two projects share the same port.
Find what's using a port: `sudo ss -tlnp | grep :8081`

**Caddy not routing correctly**

Check Caddy logs: `sudo journalctl -u caddy -n 50`
Validate the config: `sudo caddy validate --config /etc/caddy/Caddyfile`

**OCI ports not accessible despite ufw being open**

You must also open ports in the OCI Console security list — see Step 1 above.
Both the OS firewall AND the OCI network firewall must allow the traffic.
