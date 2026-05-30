# Deployment Strategy

## Goal
Host all 50 system design projects on a single public server, accessible under one domain.

## Target: Oracle Cloud Free Tier (Ampere ARM)

**Why:** 4 vCPU + 24 GB RAM, free forever. Enough headroom for 50 lightweight demo stacks running idle.

**Instance type:** `VM.Standard.A1.Flex` — 4 OCPU, 24 GB RAM, Ampere ARM  
**OS:** Ubuntu 22.04 LTS  
**Storage:** 200 GB boot volume (free tier)  
**IP:** One static public IP  
**Domain:** Point a single domain (e.g. `sysdesign.yourdomain.com`) to the server IP

### Resource estimate per project
| Resource | Per project (idle) | 50 projects |
|----------|-------------------|-------------|
| RAM | ~50–100 MB | ~2.5–5 GB |
| CPU | ~0.01 vCPU | ~0.5 vCPU |
| Ports | Internal only | Caddy handles all |

50 projects fit comfortably in 24 GB RAM with room for Prometheus + Grafana overhead.

---

## Server layout

```
/srv/sysdesign/
  shared/                   ← Caddy + shared Prometheus + shared Grafana
    docker-compose.yml
    Caddyfile
    prometheus/
    grafana/
  projects/
    01-rate-limiter/
      docker-compose.yml    ← project-specific stack
    02-url-shortener/
      docker-compose.yml
    ...
```

Each project runs its own `docker-compose.yml` on an internal port (8080, 8081, ...).  
Caddy on the host proxies all traffic with automatic HTTPS.

---

## Routing: Caddy (path-based)

All projects live under one domain with path prefixes:

```
sysdesign.yourdomain.com/rl/          → rate-limiter:8080
sysdesign.yourdomain.com/url/         → url-shortener:8081
sysdesign.yourdomain.com/paste/       → pastebin:8082
sysdesign.yourdomain.com/metrics/     → shared Grafana:3000
```

Or subdomain-per-project (requires wildcard DNS):
```
rate-limiter.yourdomain.com           → :8080
url-shortener.yourdomain.com          → :8081
```

Path-based is simpler to start with — one DNS A record, no wildcard cert needed.

**Caddyfile skeleton:**
```
sysdesign.yourdomain.com {
    handle /rl/* {
        uri strip_prefix /rl
        reverse_proxy localhost:8080
    }
    handle /url/* {
        uri strip_prefix /url
        reverse_proxy localhost:8081
    }
    # add one block per project
}
```

---

## Cloud dev/test environment (when local machine isn't enough)

For heavy builds, load tests, or running multiple stacks simultaneously before Oracle setup is ready:

### Option A: AWS EC2 Spot (cheapest, temporary)
- **Instance:** `t4g.medium` (ARM, 2 vCPU, 4 GB) or `t3.large` (x86, 2 vCPU, 8 GB)
- **Spot price:** ~$0.01–0.02/hr (vs $0.07/hr on-demand)
- **Use case:** run a project stack for a few hours, test, terminate
- **Cost for 8 hours of testing:** < $0.20

```bash
# Launch a spot instance via AWS CLI
aws ec2 run-instances \
  --image-id ami-0c55b159cbfafe1f0 \   # Ubuntu 22.04 ARM
  --instance-type t4g.medium \
  --instance-market-options '{"MarketType":"spot"}' \
  --key-name your-key \
  --security-group-ids sg-xxx \
  --count 1

# SSH in, install Docker, clone repo, run docker compose up
# Terminate when done — pay only for what you used
```

### Option B: AWS CloudShell (zero cost, zero setup)
- Free browser-based terminal with 1 GB storage, pre-installed AWS CLI
- Good for: running scripts, curl tests, inspecting logs
- Not good for: running Docker or long-lived services

### Option C: GitHub Codespaces (free 60 hrs/month)
- 2-core machine with Docker pre-installed
- Run `docker compose up` directly in the browser
- Good for: building and unit-testing projects before deploying
- Limit: 60 hrs/month on free plan, no public port exposure by default (use port forwarding)

### Option D: fly.io (free tier, persistent)
- Free 3 VMs (256 MB RAM each)
- Good for: deploying a single lightweight project (Go server only, external Redis/PG via free tiers)
- Not good for: running full stacks with Redis + Postgres + Prometheus

---

## Recommended workflow per project

```
1. Build + unit test locally (or in GitHub Codespaces)
   ↓
2. Spin up AWS Spot t4g.medium for integration test + load test
   (< $0.20 for a 8-hour session)
   ↓
3. Terminate the Spot instance
   ↓
4. Deploy to Oracle Cloud ARM (permanent showcase)
```

---

## Oracle Cloud setup checklist

- [ ] Create Oracle Cloud account (requires credit card, never charged on free tier)
- [ ] Provision `VM.Standard.A1.Flex` — 4 OCPU, 24 GB RAM, Ubuntu 22.04
- [ ] Open ports 80 and 443 in OCI Security List + Ubuntu UFW
- [ ] Install Docker + Docker Compose plugin
- [ ] Install Caddy
- [ ] Point domain A record to instance public IP
- [ ] Clone this repo to `/srv/sysdesign/`
- [ ] Deploy shared Caddy + Prometheus + Grafana stack
- [ ] Deploy projects one by one, add Caddy route per project

## Adding a new project (once server is running)

```bash
# On the Oracle server
cd /srv/sysdesign/projects/02-url-shortener
docker compose up -d

# Add one handle block to /etc/caddy/Caddyfile
# Caddy hot-reloads with:
caddy reload --config /etc/caddy/Caddyfile
```
