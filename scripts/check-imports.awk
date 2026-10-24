# Check the canonical import groups and runtime package boundaries after goimports.
function allowed(pkg, dep) {
  if (pkg == ".") return dep == "internal/app"
  if (pkg == "internal/app") return dep ~ /^(internal\/(buildinfo|launcher|nativeui)|plugins\/startup)$/
  if (pkg == "internal/nativeui") return dep ~ /^internal\/(launcher|buildinfo|nativeui\/i18n)$/
  if (pkg == "internal/launcher") return dep ~ /^internal\/(config|launcher\/(memory|proctree|textenc|vault))$/
  return 0
}
function inspect(line, name, first, group, dep) {
  sub(/^[^"]*"/, "", line)
  sub(/".*$/, "", line)
  name = line
  first = name; sub(/\/.*/, "", first)
  group = first ~ /\./ ? 1 : 0
  if (index(name, module "/") == 1) group = 2
  if (group < previous) {
    print FILENAME ": imports must be standard library, third-party, then local; run make fmt" > "/dev/stderr"
    failed = 1
  }
  previous = group
  if (FILENAME ~ /_test\.go$/) return
  if ((name == "github.com/egoist/mygo" || index(name, "github.com/egoist/mygo/") == 1) && pkg !~ /^(internal\/(app|nativeui)|plugins\/startup)$/) {
    print FILENAME ": UI dependency is not allowed here" > "/dev/stderr"; failed = 1
  }
  if (group == 2) {
    dep = substr(name, length(module) + 2)
    if (!allowed(pkg, dep)) {
      print FILENAME ": unexpected dependency on " dep > "/dev/stderr"; failed = 1
    }
  }
}
FNR == 1 { block = 0; comment = 0; header = 1; previous = 0; pkg = FILENAME; sub(/^\.\//, "", pkg); if (pkg !~ /\//) pkg = "."; else sub(/\/[^/]*$/, "", pkg) }
# goimports has normalized the source before this checker runs. Imports must
# precede declarations; never interpret raw string contents as dependencies.
!header { next }
{
  while (comment || $0 ~ /^[[:space:]]*\/\*/) {
    if (!comment) sub(/^[[:space:]]*\/\*/, "")
    comment = 1
    closeAt = index($0, "*/")
    if (!closeAt) next
    $0 = substr($0, closeAt + 2)
    comment = 0
  }
}
/^[[:space:]]*\/\// { next }
/^(const|var|type|func)([[:space:](]|$)/ { header = 0; next }
/^import[[:space:]]*\(/ { block = 1; next }
block && /^\)/ { block = 0; next }
/^import[[:space:]]+([._[:alnum:]]+[[:space:]]+)?"/ { inspect($0); next }
block && /^[[:space:]]*([._[:alnum:]]+[[:space:]]+)?"/ { inspect($0) }
END { exit failed }
