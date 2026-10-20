// xengine.rs: gorge's cross-engine bench mode for mtg-kernel's bench_kernel
// example (scripts/xengine-bench.sh in github.com/adams-shaun/gorge).
// Apache-2.0 OR MIT; written for gorge, not part of mtg-kernel.
//
//   bench_kernel --xengine-json DECK_A DECK_B WARM_S MEASURE_S SEED
//
// DECK_* are deck lists, one "N Card Name" (or "N [SET:NUM] Card Name") per
// line. Plays whole random-policy games (bench_kernel's own
// random_action_for_decision, the policy its section 2b uses), seats
// alternating, through the raw engine API: WARM_S CPU-seconds discarded, then
// MEASURE_S CPU-seconds measured. Prints one JSON line.
use super::*;

fn cpu_s() -> f64 {
    // utime + stime of this process, /proc/self/stat fields 14 and 15 (clock ticks; 100 Hz on Linux).
    let s = std::fs::read_to_string("/proc/self/stat").unwrap();
    let rest = &s[s.rfind(')').unwrap() + 2..];
    let f: Vec<&str> = rest.split_whitespace().collect();
    (f[11].parse::<f64>().unwrap() + f[12].parse::<f64>().unwrap()) / 100.0
}

fn load_deck(path: &str) -> Vec<u16> {
    let mut ids = Vec::new();
    for line in std::fs::read_to_string(path).unwrap().lines() {
        let line = line.trim();
        let Some((n, rest)) = line.split_once(' ') else { continue };
        let Ok(n) = n.parse::<usize>() else { continue };
        let mut name = rest.trim();
        if name.starts_with('[') {
            name = name[name.find(']').unwrap() + 1..].trim();
        }
        let id = card_id_by_name(name).unwrap_or_else(|| {
            println!("{{\"error\":\"card not in mtg-kernel: {name}\"}}");
            std::process::exit(3)
        });
        ids.extend(std::iter::repeat_n(id, n));
    }
    if let Err(e) = mtg_kernel::card_def::preflight_fully_supported_deck(&ids) {
        println!("{{\"error\":\"deck {path} not fully supported: {e:?}\"}}");
        std::process::exit(3);
    }
    ids
}

#[derive(Default)]
struct Tally { games: u64, turns: u64, decisions: u64, halted: u64, errors: u64, capped: u64 }

fn play(a: &[u16], b: &[u16], seed: u64, swap: bool, t: &mut Tally) {
    let mut sh = SplitMix64::seed(seed);
    let (x, y) = if swap { (b, a) } else { (a, b) };
    let lib0 = shuffled(x, &mut sh);
    let lib1 = shuffled(y, &mut sh);
    let mut state = GameState::new_from_libraries(&lib0, &lib1, debug_name, seed);
    for _ in 0..7 {
        event::propose_and_commit(&mut state, ProposedEvent::draw(PlayerId::P0));
        event::propose_and_commit(&mut state, ProposedEvent::draw(PlayerId::P1));
    }
    let mut rng = SplitMix64::seed(seed ^ 0x5EED_1234_ABCD_0001);
    let mut n = 0u64;
    loop {
        let d = engine::advance_until_decision(&mut state);
        n += 1;
        match d {
            Decision::GameOver { .. } => break,
            Decision::Halted { .. } => { t.halted += 1; break }
            _ => {}
        }
        let act = random_action_for_decision(&d, &state, &mut rng);
        let mana_id = if let Action::ActivateManaAbility(id) = act { Some(id) } else { None };
        let mut res = engine::step(&mut state, act);
        // bench_kernel's random policy was written for mono-coloured decks: a
        // dual land's mana ability needs a colour, which the engine names in
        // its (side-effect-free) refusal. Pick one of those colours at random.
        if let (Err(e), Some(id)) = (&res, mana_id) {
            if let Some(list) = e.split("explicit mana choice from [").nth(1) {
                let colors: Vec<mtg_kernel::mana::ManaColor> = list.trim_end_matches(']').split(", ")
                    .filter_map(|c| match c { "W" => Some(mtg_kernel::mana::ManaColor::W), "U" => Some(mtg_kernel::mana::ManaColor::U),
                        "B" => Some(mtg_kernel::mana::ManaColor::B), "R" => Some(mtg_kernel::mana::ManaColor::R),
                        "G" => Some(mtg_kernel::mana::ManaColor::G), "C" => Some(mtg_kernel::mana::ManaColor::C), _ => None })
                    .collect();
                if !colors.is_empty() {
                    let c = colors[rng_below(&mut rng, colors.len())];
                    res = engine::step(&mut state, Action::ActivateManaAbilityChoice(id, c));
                }
            }
        }
        if let Err(e) = res {
            t.errors += 1;
            if std::env::var("XENGINE_DEBUG").is_ok() && t.errors <= 3 {
                eprintln!("step error: {e:?}");
            }
            break;
        }
        if n >= DECISION_SAFETY_CAP || state.turn > 100 { t.capped += 1; break }
    }
    t.games += 1;
    t.turns += state.turn as u64;
    t.decisions += n;
}

pub fn run(args: &[String]) {
    let a = load_deck(&args[0]);
    let b = load_deck(&args[1]);
    let warm: f64 = args[2].parse().unwrap();
    let measure: f64 = args[3].parse().unwrap();
    let seed: u64 = args[4].parse().unwrap();
    let mut g = 0u64;
    let mut w = Tally::default();
    let c0 = cpu_s();
    while cpu_s() - c0 < warm { play(&a, &b, seed + g, g % 2 == 1, &mut w); g += 1; }
    let mut t = Tally::default();
    let (c1, w1) = (cpu_s(), Instant::now());
    while cpu_s() - c1 < measure { play(&a, &b, seed + g, g % 2 == 1, &mut t); g += 1; }
    let cpu = cpu_s() - c1;
    println!("{{\"engine\":\"mtg-kernel\",\"policy\":\"random\",\"warmup_games\":{},\"games\":{},\"turns\":{},\"decisions\":{},\"halted\":{},\"errors\":{},\"capped\":{},\"cpu_s\":{:.3},\"wall_s\":{:.3}}}",
        w.games, t.games, t.turns, t.decisions, t.halted, t.errors, t.capped, cpu, w1.elapsed().as_secs_f64());
}
