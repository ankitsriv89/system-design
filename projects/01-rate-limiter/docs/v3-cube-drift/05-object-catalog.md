# Space Object Catalog (`internal/cube/catalog.go`)

## Purpose

Maps the 1000 cells of the cube to a curated set of ~70 real space objects, organized
by difficulty zone (distance from Earth). Each cell resolves to one object deterministically
(same cell always → same object → same quiz topic). Questions are loaded from a seeded
question bank; Groq generates new ones on demand and caches them to Postgres.

## Go types

```go
type SpaceObject struct {
    CatalogID     string  // unique slug, e.g. "europa", "trappist-1e", "sgr-a"
    Name          string  // display name
    Kind          string  // "planet"|"moon"|"asteroid"|"comet"|"dwarf"|"star"|"exoplanet"|"nebula"|"blackhole"
    QuestionTopic string  // passed to Groq, e.g. "Europa, its subsurface ocean, and astrobiology"
    Zone          int     // 0..3 (difficulty, by distance-from-Earth)
}
```

## Zone mapping (by D_from_start = x+y+z)

| Zone | D_from_start | Region | Difficulty |
|---|---|---|---|
| 0 | 0 – 6 | Inner solar system | Easy |
| 1 | 7 – 13 | Outer solar system | Medium |
| 2 | 14 – 20 | Nearby stars & exoplanets | Hard |
| 3 | 21 – 27 | Deep space / galactic core | Expert |

```go
func ZoneOf(c Coord) int {
    d := c.DistFromStart()  // 0..27
    switch {
    case d <= 6:  return 0
    case d <= 13: return 1
    case d <= 20: return 2
    default:      return 3
    }
}
```

## Curated catalog (~70 objects)

### Zone 0 — Inner solar system (easy)
| CatalogID | Name | Kind | Topic |
|---|---|---|---|
| `mercury` | Mercury | planet | Mercury's orbit, surface, and extreme temperature swings |
| `venus` | Venus | planet | Venus's thick atmosphere, greenhouse effect, and surface pressure |
| `earth` | Earth | planet | Earth science: oceans, atmosphere, magnetic field |
| `moon` | Moon | moon | The Moon, Apollo missions, and lunar geology |
| `mars` | Mars | planet | Mars, Olympus Mons, and rover exploration |
| `phobos` | Phobos | moon | Phobos, its origin, and predicted collision with Mars |
| `deimos` | Deimos | moon | Deimos and the small moons of Mars |
| `ceres` | Ceres | dwarf | Ceres, the largest asteroid belt object, and the Dawn mission |
| `vesta` | Vesta | asteroid | Vesta, its massive crater, and HED meteorites |
| `pallas` | Pallas | asteroid | Pallas and the diversity of large asteroids |
| `eros` | Eros | asteroid | Eros and the NEAR Shoemaker mission |
| `itokawa` | Itokawa | asteroid | Itokawa and the Hayabusa sample-return mission |
| `bennu` | Bennu | asteroid | Bennu, its composition, and the OSIRIS-REx mission |
| `ryugu` | Ryugu | asteroid | Ryugu and the Hayabusa2 sample return |
| `halley` | Halley's Comet | comet | Halley's Comet, its orbit, and historical observations |

### Zone 1 — Outer solar system (medium)
| CatalogID | Name | Kind | Topic |
|---|---|---|---|
| `jupiter` | Jupiter | planet | Jupiter's Great Red Spot, composition, and magnetosphere |
| `io` | Io | moon | Io's volcanic activity and tidal heating by Jupiter |
| `europa` | Europa | moon | Europa's subsurface ocean and astrobiology potential |
| `ganymede` | Ganymede | moon | Ganymede, the largest moon in the solar system |
| `callisto` | Callisto | moon | Callisto's ancient cratered surface |
| `saturn` | Saturn | planet | Saturn's rings, density, and ring system composition |
| `titan` | Titan | moon | Titan's nitrogen atmosphere and methane lakes |
| `enceladus` | Enceladus | moon | Enceladus's water geysers and potential for life |
| `mimas` | Mimas | moon | Mimas and its massive Herschel Crater |
| `rhea` | Rhea | moon | Rhea and Saturn's icy mid-sized moons |
| `uranus` | Uranus | planet | Uranus's axial tilt, ice giant composition, and rings |
| `miranda` | Miranda | moon | Miranda's extreme terrain and Verona Rupes cliff |
| `neptune` | Neptune | planet | Neptune's supersonic winds and the Great Dark Spot |
| `triton` | Triton | moon | Triton's retrograde orbit and nitrogen geysers |
| `pluto` | Pluto | dwarf | Pluto, Tombaugh Regio, and the New Horizons flyby |
| `charon` | Charon | moon | Charon and the Pluto–Charon binary system |
| `eris` | Eris | dwarf | Eris, the most massive dwarf planet, and Dysnomia |
| `makemake` | Makemake | dwarf | Makemake and the Kuiper Belt's classical objects |
| `haumea` | Haumea | dwarf | Haumea's egg shape, fast rotation, and ring |
| `67p` | 67P / Churyumov–Gerasimenko | comet | 67P and the Rosetta/Philae mission |
| `oort-cloud` | Oort Cloud | asteroid | The Oort Cloud, long-period comets, and its hypothetical extent |

### Zone 2 — Nearby stars & exoplanets (hard)
| CatalogID | Name | Kind | Topic |
|---|---|---|---|
| `proxima-centauri` | Proxima Centauri | star | Proxima Centauri, red dwarfs, and stellar flares |
| `proxima-b` | Proxima Centauri b | exoplanet | Proxima b in the habitable zone of Proxima Centauri |
| `alpha-centauri-a` | Alpha Centauri A | star | Alpha Centauri A, a G-type star similar to the Sun |
| `alpha-centauri-b` | Alpha Centauri B | star | Alpha Centauri B and binary star systems |
| `barnards-star` | Barnard's Star | star | Barnard's Star, the fastest-moving star in the sky |
| `sirius` | Sirius | star | Sirius, the brightest star, and its white dwarf companion |
| `trappist-1` | TRAPPIST-1 | star | TRAPPIST-1 and its system of seven rocky exoplanets |
| `trappist-1e` | TRAPPIST-1e | exoplanet | TRAPPIST-1e and habitability in ultra-cool dwarf systems |
| `kepler-442b` | Kepler-442b | exoplanet | Kepler-442b and the Earth Similarity Index |
| `tau-ceti-e` | Tau Ceti e | exoplanet | Tau Ceti, its debris disk, and potentially habitable planets |
| `betelgeuse` | Betelgeuse | star | Betelgeuse's size, variability, and future supernova |
| `rigel` | Rigel | star | Rigel, a blue supergiant in Orion |
| `vega` | Vega | star | Vega's debris disk and its use as a photometric standard |
| `wolf-359` | Wolf 359 | star | Wolf 359 and M-dwarf flare stars |
| `orion-nebula` | Orion Nebula | nebula | The Orion Nebula as a stellar nursery and star-formation region |
| `eagle-nebula` | Eagle Nebula | nebula | The Eagle Nebula and the Pillars of Creation |
| `crab-nebula` | Crab Nebula | nebula | The Crab Nebula, its pulsar, and the 1054 supernova |
| `helix-nebula` | Helix Nebula | nebula | The Helix Nebula as a planetary nebula and white dwarf remnant |
| `voyager-1` | Voyager 1 | asteroid | Voyager 1 in interstellar space and the heliopause |

### Zone 3 — Deep space / galactic core (expert)
| CatalogID | Name | Kind | Topic |
|---|---|---|---|
| `sgr-a` | Sagittarius A* | blackhole | Sagittarius A*, its mass, and the Event Horizon Telescope image |
| `m87-star` | M87* | blackhole | M87* and the first-ever image of a black hole's event horizon |
| `cygnus-x1` | Cygnus X-1 | blackhole | Cygnus X-1, the first confirmed stellar-mass black hole |
| `andromeda` | Andromeda Galaxy | nebula | The Andromeda Galaxy and its future collision with the Milky Way |
| `magellanic-clouds` | Magellanic Clouds | nebula | The Large and Small Magellanic Clouds as satellite galaxies |
| `globular-cluster-47tuc` | 47 Tucanae | nebula | 47 Tucanae, a dense globular cluster, and millisecond pulsars |
| `crab-pulsar` | Crab Pulsar | star | The Crab Pulsar, neutron stars, and pulsar timing |
| `magnetar-1806` | SGR 1806-20 | star | Magnetars, their extreme magnetic fields, and starquakes |
| `gr-waves` | GW150914 source | blackhole | Gravitational waves, LIGO, and the first binary black hole merger |
| `dark-matter` | Dark Matter halo | blackhole | Dark matter, galactic rotation curves, and detection attempts |
| `cosmic-background` | Cosmic Microwave Background | nebula | The CMB, the Big Bang's afterglow, and early universe cosmology |

## Cell → object resolution

```go
// ObjectForCell deterministically maps a cell to the nearest catalog object
// in the same zone. Same cell always returns the same object.
func ObjectForCell(c Coord) SpaceObject {
    zone := ZoneOf(c)
    pool := objectsByZone[zone]    // pre-built slice of objects in this zone

    // Stable hash: XOR of position * distinct primes, mod pool size.
    // No randomness — same cell always maps to same object.
    h := uint64(c.X)*2654435761 ^ uint64(c.Y)*2246822519 ^ uint64(c.Z)*3266489917
    return pool[h % uint64(len(pool))]
}
```

## Question resolution (reusing v1 pattern)

```go
// In cube/store.go CubeStore.GetQuestion(ctx, objectID string) (*cube.Question, error):
q, err := s.db.QueryRow(`
    SELECT id, question, choices, answer, hint, source
    FROM cube_questions
    WHERE object_id = $1
    ORDER BY random() LIMIT 1`, objectID)
if errors.Is(err, sql.ErrNoRows) && groq != nil {
    obj := CatalogByID[objectID]
    // Shim in groq_topic.go:
    dest := odyssey.Destination{ID: obj.CatalogID, Topic: obj.QuestionTopic}
    q, err = groq.GenerateQuestion(ctx, dest)
    // Save to cube_questions with source="groq"
}
return q, err
```

## Seeding

`CubeStore.SeedObjects(ctx)` — idempotent, runs at startup:
1. Insert all ~70 catalog objects into `cube_objects` (ON CONFLICT DO NOTHING).
2. Insert pre-written seed questions for Zone 0 and Zone 1 objects (3–5 per object)
   into `cube_questions` — reduces cold-cache Groq calls for the most-visited cells.
   Zone 2 and Zone 3 questions are generated by Groq on first visit.
