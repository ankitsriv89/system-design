package odyssey

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
)

// Question is a row in the odyssey_questions table.
type Question struct {
	ID            int64
	DestinationID string
	Question      string
	Choices       json.RawMessage // JSON array of 4 strings
	Answer        int
	Hint          string
	Source        string // "seed" | "groq"
	Topic         string
}

// Store manages the odyssey_questions and odyssey_progress tables.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS odyssey_questions (
	id             BIGSERIAL PRIMARY KEY,
	destination_id TEXT        NOT NULL,
	question       TEXT        NOT NULL,
	choices        JSONB       NOT NULL,
	answer         SMALLINT    NOT NULL CHECK (answer BETWEEN 0 AND 3),
	hint           TEXT        NOT NULL DEFAULT '',
	source         TEXT        NOT NULL DEFAULT 'seed',
	topic          TEXT        NOT NULL DEFAULT '',
	created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS odyssey_progress (
	ip              TEXT        PRIMARY KEY,
	destination_idx INTEGER     NOT NULL DEFAULT 0,
	hops_used       INTEGER     NOT NULL DEFAULT 0,
	completed       BOOLEAN     NOT NULL DEFAULT false,
	last_activity   TIMESTAMPTZ NOT NULL DEFAULT now()
);
`)
	return err
}

// RandomQuestion returns a random question for the given destination.
// Returns sql.ErrNoRows if none exist.
func (s *Store) RandomQuestion(ctx context.Context, destinationID string) (*Question, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, destination_id, question, choices, answer, hint, source, topic
		FROM   odyssey_questions
		WHERE  destination_id = $1
		ORDER  BY random()
		LIMIT  1
	`, destinationID)
	return scanQuestion(row)
}

// RandomQuestionExcept returns a random question excluding one already seen.
func (s *Store) RandomQuestionExcept(ctx context.Context, destinationID string, excludeID int64) (*Question, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, destination_id, question, choices, answer, hint, source, topic
		FROM   odyssey_questions
		WHERE  destination_id = $1 AND id != $2
		ORDER  BY random()
		LIMIT  1
	`, destinationID, excludeID)
	return scanQuestion(row)
}

func scanQuestion(row *sql.Row) (*Question, error) {
	var q Question
	if err := row.Scan(&q.ID, &q.DestinationID, &q.Question, &q.Choices, &q.Answer, &q.Hint, &q.Source, &q.Topic); err != nil {
		return nil, err
	}
	return &q, nil
}

// SaveQuestion inserts a question (Groq-generated or seeded) and returns its ID.
func (s *Store) SaveQuestion(ctx context.Context, q *Question) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO odyssey_questions (destination_id, question, choices, answer, hint, source, topic)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, q.DestinationID, q.Question, q.Choices, q.Answer, q.Hint, q.Source, q.Topic).Scan(&id)
	return id, err
}

// GetProgress loads a player's progress row; returns a zero-value State for new players
// so callers can treat first-time visitors the same as returning ones.
// The "ip" column stores the session ID (not the raw IP) — the name is a historical artifact.
func (s *Store) GetProgress(ctx context.Context, ip string) (*State, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT ip, destination_idx, hops_used, completed, last_activity
		FROM   odyssey_progress
		WHERE  ip = $1
	`, ip)
	var st State
	err := row.Scan(&st.IP, &st.DestinationIdx, &st.HopsUsed, &st.Completed, &st.LastActivity)
	if err == sql.ErrNoRows {
		st.IP = ip
		return &st, nil
	}
	return &st, err
}

// SaveProgress upserts a player's progress.
func (s *Store) SaveProgress(ctx context.Context, st *State) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO odyssey_progress (ip, destination_idx, hops_used, completed, last_activity)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (ip) DO UPDATE SET
			destination_idx = EXCLUDED.destination_idx,
			hops_used       = EXCLUDED.hops_used,
			completed       = EXCLUDED.completed,
			last_activity   = now()
	`, st.IP, st.DestinationIdx, st.HopsUsed, st.Completed)
	return err
}

// GetQuestionByID loads a single question by primary key.
func (s *Store) GetQuestionByID(ctx context.Context, id int64) (*Question, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, destination_id, question, choices, answer, hint, source, topic
		FROM   odyssey_questions
		WHERE  id = $1
	`, id)
	return scanQuestion(row)
}

// CountQuestions returns how many questions exist for a destination.
func (s *Store) CountQuestions(ctx context.Context, destinationID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM odyssey_questions WHERE destination_id = $1`, destinationID,
	).Scan(&n)
	return n, err
}

// SeedQuestions inserts the built-in question bank if the table is empty.
// Idempotent: a non-zero row count means a previous seed (or user-added questions)
// already exists, so we skip to avoid duplicates on every container restart.
func (s *Store) SeedQuestions(ctx context.Context) error {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM odyssey_questions`).Scan(&total); err != nil {
		return err
	}
	if total > 0 {
		return nil
	}

	questions := builtinQuestions()
	// shuffle so ORDER BY random() gets variety even without the random() clause
	rand.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO odyssey_questions (destination_id, question, choices, answer, hint, source, topic)
		VALUES ($1, $2, $3, $4, $5, 'seed', $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, q := range questions {
		choices, err := json.Marshal(q.choices)
		if err != nil {
			return fmt.Errorf("marshal choices: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, q.destID, q.question, choices, q.answer, q.hint, q.topic); err != nil {
			return fmt.Errorf("insert seed question: %w", err)
		}
	}
	return tx.Commit()
}

// seedQ is a compact form used only in builtinQuestions().
type seedQ struct {
	destID   string
	topic    string
	question string
	choices  []string
	answer   int
	hint     string
}

func builtinQuestions() []seedQ {
	return []seedQ{
		// Earth
		{"earth", "Earth science", "What percentage of Earth's surface is covered by water?", []string{"A. 51%", "B. 61%", "C. 71%", "D. 81%"}, 2, "Think of the phrase 'the blue planet' — most of it is ocean."},
		{"earth", "Earth science", "What is the name of the single supercontinent that existed ~300 million years ago?", []string{"A. Gondwana", "B. Laurasia", "C. Pangaea", "D. Rodinia"}, 2, "Greek for 'all lands' — it broke apart to form the continents we know today."},
		{"earth", "Earth science", "What is Earth's inner core primarily composed of?", []string{"A. Liquid iron and nickel", "B. Solid iron and nickel", "C. Silicon and magnesium", "D. Uranium and thorium"}, 1, "Despite enormous heat, the pressure at the center keeps it in a solid state."},
		{"earth", "Earth science", "Approximately how old is Earth?", []string{"A. 2.5 billion years", "B. 4.5 billion years", "C. 6.5 billion years", "D. 8.5 billion years"}, 1, "Radiometric dating of meteorites and Earth's oldest rocks converge on a well-known figure."},
		{"earth", "Earth science", "Which layer of the atmosphere contains the ozone layer?", []string{"A. Troposphere", "B. Mesosphere", "C. Thermosphere", "D. Stratosphere"}, 3, "It sits above where weather happens, between roughly 15 and 35 km altitude."},

		// Moon
		{"moon", "lunar science", "How long does it take the Moon to orbit Earth?", []string{"A. ~14 days", "B. ~27 days", "C. ~365 days", "D. ~7 days"}, 1, "The Moon's orbital period and rotation period are the same — which is why we always see one side."},
		{"moon", "Apollo missions", "Who was the first human to walk on the Moon?", []string{"A. Buzz Aldrin", "B. Michael Collins", "C. Neil Armstrong", "D. Pete Conrad"}, 2, "He described it as 'one small step for man, one giant leap for mankind.'"},
		{"moon", "lunar science", "What causes the Moon to have phases as seen from Earth?", []string{"A. Earth's shadow on the Moon", "B. The Moon's rotation speed", "C. The angle of sunlight on the visible portion of the Moon", "D. Clouds on the Moon"}, 2, "It's not an eclipse — phases happen because of geometry between the Sun, Moon, and Earth."},
		{"moon", "lunar science", "Which Apollo mission was the first to land humans on the Moon?", []string{"A. Apollo 8", "B. Apollo 10", "C. Apollo 11", "D. Apollo 13"}, 2, "It landed in the Sea of Tranquility in July 1969."},
		{"moon", "lunar science", "The Moon's surface gravity is approximately what fraction of Earth's?", []string{"A. 1/2", "B. 1/4", "C. 1/6", "D. 1/10"}, 2, "An astronaut weighing 180 lbs on Earth would weigh about 30 lbs here."},

		// Mars
		{"mars", "Mars exploration", "What is the name of the largest volcano in the solar system, located on Mars?", []string{"A. Mauna Kea", "B. Olympus Mons", "C. Elysium Mons", "D. Ascraeus Mons"}, 1, "It's roughly the size of France and rises ~22 km above the Martian surface."},
		{"mars", "Mars missions", "Which NASA rover landed on Mars in February 2021?", []string{"A. Curiosity", "B. Opportunity", "C. Perseverance", "D. Spirit"}, 2, "It landed in Jezero Crater and carried the Ingenuity helicopter."},
		{"mars", "Mars science", "What gives Mars its reddish color?", []string{"A. Sulfur deposits", "B. Iron oxide (rust) in the soil", "C. Red clay minerals", "D. Dried volcanic lava"}, 1, "The surface is covered in a fine iron-rich dust that has oxidized over billions of years."},
		{"mars", "Mars science", "How long is a Martian day (sol)?", []string{"A. 22 hours 32 minutes", "B. 24 hours 37 minutes", "C. 26 hours 10 minutes", "D. 28 hours 00 minutes"}, 1, "Remarkably close to Earth's — one reason Mars is considered potentially habitable."},
		{"mars", "Mars moons", "What are the two moons of Mars called?", []string{"A. Titan and Triton", "B. Io and Europa", "C. Phobos and Deimos", "D. Charon and Nix"}, 2, "Their names mean Fear and Dread — they are the sons of Ares in Greek mythology."},

		// Asteroid Belt
		{"asteroid-belt", "solar system formation", "Between which two planets is the asteroid belt located?", []string{"A. Earth and Mars", "B. Mars and Jupiter", "C. Jupiter and Saturn", "D. Saturn and Uranus"}, 1, "Jupiter's massive gravity prevented this material from forming a planet."},
		{"asteroid-belt", "asteroids", "What is the largest object in the asteroid belt?", []string{"A. Vesta", "B. Pallas", "C. Hygiea", "D. Ceres"}, 3, "It's large enough to be classified as a dwarf planet and was visited by Dawn spacecraft."},
		{"asteroid-belt", "asteroids", "What is the approximate total mass of all objects in the asteroid belt?", []string{"A. About the same as Earth", "B. About 4% of the Moon's mass", "C. About half of Mars' mass", "D. About 10 times Earth's mass"}, 1, "The belt looks dense in movies but is actually mostly empty space."},
		{"asteroid-belt", "asteroids", "Which spacecraft was the first to fly through the asteroid belt?", []string{"A. Voyager 1", "B. Cassini", "C. Pioneer 10", "D. New Horizons"}, 2, "It proved that the belt wasn't the dangerous obstacle science fiction imagined."},
		{"asteroid-belt", "asteroids", "What type of asteroid is the most common in the outer part of the belt?", []string{"A. S-type (silicate)", "B. M-type (metallic)", "C. C-type (carbonaceous)", "D. V-type (volcanic)"}, 2, "Dark, carbon-rich, and thought to be remnants from the early solar system."},

		// Jupiter
		{"jupiter", "Jupiter", "How many Earth-sized planets could fit inside Jupiter?", []string{"A. ~100", "B. ~500", "C. ~1000", "D. ~1300"}, 3, "Jupiter is by far the largest planet — more than twice as massive as all other planets combined."},
		{"jupiter", "Jupiter moons", "Which of Jupiter's moons is the most volcanically active body in the solar system?", []string{"A. Europa", "B. Ganymede", "C. Io", "D. Callisto"}, 2, "Tidal heating from Jupiter's gravity keeps its interior molten."},
		{"jupiter", "Jupiter science", "What is the Great Red Spot?", []string{"A. A massive impact crater", "B. A persistent anticyclonic storm", "C. A region of iron-rich clouds", "D. A volcanic region"}, 1, "It has been observed for at least 350 years and is shrinking over time."},
		{"jupiter", "Jupiter moons", "Which moon of Jupiter is thought to have a subsurface ocean beneath its icy crust?", []string{"A. Io", "B. Callisto", "C. Europa", "D. Amalthea"}, 2, "It's one of the top candidates in the search for extraterrestrial life."},
		{"jupiter", "Jupiter science", "Jupiter emits more energy than it receives from the Sun. What is the primary reason?", []string{"A. Nuclear fusion in its core", "B. Slow gravitational contraction releasing heat", "C. Radioactive decay", "D. Tidal friction with its moons"}, 1, "It's still slowly shrinking from its formation, releasing gravitational potential energy as heat."},

		// Saturn
		{"saturn", "Saturn", "What are Saturn's rings primarily composed of?", []string{"A. Dust and gas", "B. Silicate rock fragments", "C. Ice particles and rocky debris", "D. Methane crystals"}, 2, "The particles range from tiny grains to chunks as large as a house."},
		{"saturn", "Saturn moons", "Which moon of Saturn has a thick nitrogen atmosphere and liquid methane lakes?", []string{"A. Enceladus", "B. Titan", "C. Mimas", "D. Rhea"}, 1, "The Huygens probe landed on it in 2005, the most distant landing ever made."},
		{"saturn", "Saturn science", "Saturn's density is so low that it could theoretically float in water. What is its average density?", []string{"A. 0.5 g/cm³", "B. 0.7 g/cm³", "C. 1.2 g/cm³", "D. 1.8 g/cm³"}, 1, "Water's density is 1 g/cm³ — Saturn is the only planet less dense than water."},
		{"saturn", "Saturn moons", "The Cassini spacecraft discovered geysers of water vapor erupting from which moon of Saturn?", []string{"A. Titan", "B. Dione", "C. Enceladus", "D. Tethys"}, 2, "It shoots water ice from its south pole, feeding one of Saturn's rings."},
		{"saturn", "Saturn science", "How long is one day on Saturn?", []string{"A. ~6 hours", "B. ~10.5 hours", "C. ~16 hours", "D. ~24 hours"}, 1, "Gas giants rotate very fast — Saturn spins faster than Earth despite being 9× wider."},

		// Uranus
		{"uranus", "Uranus", "Uranus rotates on its side with an axial tilt of approximately:", []string{"A. 23 degrees", "B. 45 degrees", "C. 65 degrees", "D. 98 degrees"}, 3, "It essentially rolls around the Sun on its side — possibly from a giant ancient collision."},
		{"uranus", "Uranus", "What gives Uranus its blue-green color?", []string{"A. Liquid water in the upper atmosphere", "B. Methane gas absorbing red light", "C. Ammonia ice crystals", "D. Sulfur compounds"}, 1, "Methane in the atmosphere absorbs red wavelengths and reflects blue-green light."},
		{"uranus", "Uranus science", "Uranus and Neptune are classified as what type of planet?", []string{"A. Gas giants", "B. Terrestrial planets", "C. Ice giants", "D. Super-Earths"}, 2, "Their interiors are dominated by ices of water, methane, and ammonia rather than hydrogen gas."},
		{"uranus", "Uranus science", "How many known moons does Uranus have (as of recent counts)?", []string{"A. 5", "B. 18", "C. 27", "D. 53"}, 2, "All named after characters from Shakespeare and Alexander Pope."},
		{"uranus", "Uranus", "Which spacecraft is the only one to have visited Uranus?", []string{"A. Pioneer 11", "B. Voyager 1", "C. Cassini", "D. Voyager 2"}, 3, "It flew by in 1986 and is still the only close-up look we've had."},

		// Neptune
		{"neptune", "Neptune", "What is the name of Neptune's largest moon, which orbits in the opposite direction of Neptune's rotation?", []string{"A. Proteus", "B. Triton", "C. Nereid", "D. Larissa"}, 1, "Its retrograde orbit suggests it was captured from the Kuiper Belt."},
		{"neptune", "Neptune science", "Neptune has the strongest winds in the solar system. How fast can they reach?", []string{"A. ~500 km/h", "B. ~1,000 km/h", "C. ~2,100 km/h", "D. ~3,500 km/h"}, 2, "Faster than the speed of sound on Earth, despite being so far from the Sun's heat."},
		{"neptune", "Neptune", "Neptune was the first planet to be predicted mathematically before it was directly observed. Who made the prediction?", []string{"A. Galileo Galilei", "B. Johann Galle and Urbain Le Verrier", "C. William Herschel", "D. Edmond Halley"}, 1, "Le Verrier's calculations of Uranus's orbital wobble led directly to Neptune's discovery in 1846."},
		{"neptune", "Neptune science", "What is the Great Dark Spot observed on Neptune by Voyager 2 in 1989?", []string{"A. A permanent storm similar to Jupiter's Great Red Spot", "B. A transient storm system that had disappeared by 1994", "C. A large impact crater", "D. A region of dark methane ice"}, 1, "Unlike Jupiter's storm, Neptune's Great Dark Spot vanished within a few years."},
		{"neptune", "Neptune", "How long does it take Neptune to complete one orbit around the Sun?", []string{"A. ~84 years", "B. ~116 years", "C. ~165 years", "D. ~248 years"}, 2, "It has only completed one full orbit since its discovery in 1846."},

		// Pluto
		{"pluto", "Pluto and Kuiper Belt", "In what year was Pluto reclassified as a dwarf planet?", []string{"A. 1999", "B. 2003", "C. 2006", "D. 2015"}, 2, "The International Astronomical Union created the formal definition of 'planet' that same year."},
		{"pluto", "New Horizons mission", "NASA's New Horizons spacecraft made its historic flyby of Pluto in which year?", []string{"A. 2010", "B. 2013", "C. 2015", "D. 2018"}, 2, "It revealed Tombaugh Regio — the heart-shaped nitrogen ice plain that surprised everyone."},
		{"pluto", "Pluto science", "What is the name of Pluto's largest moon?", []string{"A. Nix", "B. Hydra", "C. Charon", "D. Styx"}, 2, "It's so large relative to Pluto that the two are considered a double dwarf planet system."},
		{"pluto", "Kuiper Belt", "The Kuiper Belt is a region of the solar system beyond Neptune, extending from ~30 AU to:", []string{"A. ~50 AU", "B. ~100 AU", "C. ~200 AU", "D. ~1000 AU"}, 0, "Beyond that lies the scattered disc and eventually the Oort Cloud."},
		{"pluto", "Pluto science", "Pluto's orbit is unusual in that it is:", []string{"A. Perfectly circular", "B. Highly elliptical and inclined 17° to the ecliptic", "C. Retrograde like Venus", "D. Perpendicular to the ecliptic"}, 1, "At perihelion it is actually closer to the Sun than Neptune."},

		// Alpha Centauri
		{"alpha-centauri", "nearby stars", "How many stars make up the Alpha Centauri system?", []string{"A. 1", "B. 2", "C. 3", "D. 4"}, 2, "Alpha Centauri A, Alpha Centauri B, and Proxima Centauri — though Proxima may not be gravitationally bound."},
		{"alpha-centauri", "exoplanets", "An Earth-sized planet orbiting in the habitable zone of Proxima Centauri is called:", []string{"A. Alpha Cen Bb", "B. Proxima b", "C. Proxima d", "D. Centauri Prime"}, 1, "Discovered in 2016, it's one of the most discussed targets for future interstellar missions."},
		{"alpha-centauri", "interstellar travel", "At the speed of current fastest spacecraft (~17 km/s), how long would it take to reach Alpha Centauri?", []string{"A. ~700 years", "B. ~4,000 years", "C. ~75,000 years", "D. ~1,000,000 years"}, 2, "Voyager 1 travels at about 17 km/s — it would take roughly 75,000 years."},
		{"alpha-centauri", "nearby stars", "Proxima Centauri is the closest star to the Sun. What type of star is it?", []string{"A. Yellow dwarf (G-type)", "B. White dwarf", "C. Red dwarf (M-type)", "D. Orange subgiant (K-type)"}, 2, "Red dwarfs are the most common type of star in the Milky Way."},
		{"alpha-centauri", "interstellar travel", "Project Breakthrough Starshot aims to send laser-propelled probes to Alpha Centauri in:", []string{"A. ~5 years", "B. ~20 years", "C. ~200 years", "D. ~1,000 years"}, 1, "The probes would travel at ~20% the speed of light using a giant laser sail."},

		// Sagittarius A*
		{"sgr-a", "black holes", "What type of object is Sagittarius A*?", []string{"A. A neutron star", "B. A supermassive black hole", "C. A globular cluster", "D. A pulsar"}, 1, "It contains the mass of ~4 million Suns compressed into a region smaller than our solar system."},
		{"sgr-a", "galactic centers", "In which direction (constellation) does Sagittarius A* lie from Earth?", []string{"A. Orion", "B. Centaurus", "C. Sagittarius", "D. Scorpius"}, 2, "The galactic center lies in the direction of Sagittarius, behind vast dust clouds."},
		{"sgr-a", "black holes", "The first image of Sagittarius A* was released in which year?", []string{"A. 2019", "B. 2021", "C. 2022", "D. 2024"}, 2, "Captured by the Event Horizon Telescope — the same collaboration that imaged M87* in 2019."},
		{"sgr-a", "black holes", "What happens to time near the event horizon of a black hole, according to general relativity?", []string{"A. Time speeds up", "B. Time slows down (gravitational time dilation)", "C. Time stops permanently", "D. Time is unaffected"}, 1, "Extreme gravity curves spacetime — a clock near a black hole ticks slower than one far away."},
		{"sgr-a", "Milky Way", "How long does it take the Sun to complete one orbit around the Milky Way's center?", []string{"A. ~2.5 million years", "B. ~25 million years", "C. ~225 million years", "D. ~2.25 billion years"}, 2, "Called a 'galactic year' — Earth has completed about 20 of them in its lifetime."},
	}
}
