# govulncheck v1.9.0 protocol v1.0.0: validate before making any gate decision.
def require($condition; $message):
  if $condition then . else error($message) end;
def text: type == "string" and test("\\S");
. as $messages
| require(length > 0; "empty JSON stream")
| require(all(.[];
    type == "object" and (. as $message
      | [keys[] | select(. == "config" or . == "SBOM" or . == "osv" or . == "finding" or . == "progress")] as $kinds
      | ($kinds | length) == 1 and ($message[$kinds[0]] | type == "object")));
    "expected one protocol message kind per object")
| require(.[0].config != null; "missing initial config")
| [.[].config | select(. != null)] as $configs
| require(($configs | length) == 1; "expected exactly one config")
| require($configs[0] | .protocol_version == "v1.0.0"
    and .scanner_name == "govulncheck" and .scanner_version == "v1.9.0"
    and .scan_level == "symbol" and .scan_mode == "source";
    "unexpected scanner config")
| [$messages[].SBOM | select(. != null)] as $sboms
| require(($sboms | length) == 1; "expected exactly one SBOM")
| $sboms[0] as $sbom
| require($sbom.roots | type == "array" and length > 0 and all(.[]; text); "missing SBOM roots")
| require($sbom.modules | type == "array" and length > 0
    and all(.[]; type == "object" and (.path | text)
      and (if has("version") then (.version | type == "string") else true end));
    "invalid SBOM modules")
| [$messages[].osv | select(. != null)] as $osvs
| require(all($osvs[]; .id | text); "missing OSV id")
| [$messages[].finding | select(. != null)] as $findings
| require(all($findings[];
    . as $finding
    | (.osv | text) and any($osvs[]; .id == $finding.osv)
    and (.trace | type == "array" and length > 0)
    and all(.trace[];
      . as $frame | type == "object" and (.module | text)
      and all(["version", "package", "function", "receiver"][];
        . as $key | if $frame | has($key) then $frame[$key] | text else true end)
      and (if has("function") then has("package") else true end)
      and (if has("receiver") then has("function") else true end)
      and (if has("position") then .position | type == "object"
        and (. as $position | all(["offset", "line", "column"][];
          $position[.] | type == "number" and . >= 0 and floor == .))
        and (if has("filename") then .filename | type == "string" else true end)
        else true end))
    and (.trace[0] as $first
      | any($sbom.modules[]; .path == $first.module and (.version // "") == ($first.version // "")))
    and (if .trace[0] | has("function") then true
      else (.trace | length) == 1 and (.trace[0] | has("position") | not) end));
    "invalid finding trace or missing OSV/SBOM reference")
# The first trace frame determines the level, not arbitrary later call frames.
| [$findings[] | .trace[0] as $first | {
    key: [.osv, $first.module, ($first.version // "")],
    level: (if $first.function then "symbol" elif $first.package then "package" else "module" end)
  }] | unique as $levels
| [$levels[] | select(.level == "symbol") | .key] | unique as $symbols
| [$levels[] | select(.level == "package") | .key] | unique - $symbols as $packages
| [$levels[] | select(.level == "module") | .key] | unique - $symbols - $packages as $modules
# Exact audited legacy dependency only; these are not blanket suppressions.
| ($symbols - [
    ["GO-2026-4887", "github.com/docker/docker", "v28.5.2+incompatible"],
    ["GO-2026-4883", "github.com/docker/docker", "v28.5.2+incompatible"]
  ]) as $unexpected
| def describe: if length == 0 then "none" else map("\(.[0]) (\(.[1])@\(.[2]))") | join(", ") end;
  "Module-only inventory (not symbol calls): \($modules | describe)",
  "Package-only inventory (not symbol calls): \($packages | describe)",
  (if ($unexpected | length) > 0 then error("Unaccepted symbol findings: \($unexpected | describe)")
   elif ($symbols | length) > 0 then
     "Symbol gate accepted pre-approved findings (NOT a vulnerability-free scan): \($symbols | describe)"
   else "Symbol gate passed: no symbol findings." end)
