# Wires xengine.rs into mtg-kernel's examples/bench_kernel.rs (run from the mtg-kernel crate dir).
import sys, shutil
p = "examples/bench_kernel.rs"
s = open(p).read()
if "mod xengine;" not in s:
    s = s.replace("fn main() {\n    let args: Vec<String> = std::env::args().skip(1).collect();\n",
        "#[path = \"bench_kernel/xengine.rs\"]\nmod xengine;\n\nfn main() {\n    let args: Vec<String> = std::env::args().skip(1).collect();\n"
        "    if args.first().map(String::as_str) == Some(\"--xengine-json\") {\n        xengine::run(&args[1..]);\n        return;\n    }\n", 1)
    assert "mod xengine;" in s
    open(p, "w").write(s)
shutil.copy(sys.argv[1], "examples/bench_kernel/xengine.rs")
