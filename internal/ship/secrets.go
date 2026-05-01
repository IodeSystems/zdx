package ship

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadSecretsFile parses a Java-style .properties file (key=value per line)
// and returns its contents as a map. Lines starting with '#' or '!' are
// treated as comments; blank lines are skipped. A missing file returns an
// empty map and a nil error so callers can treat the secrets file as
// optional.
func LoadSecretsFile(path string) (map[string]string, error) {
	out := make(map[string]string)
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineno := 0
	for sc.Scan() {
		lineno++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			return nil, fmt.Errorf("%s:%d: malformed line (expected key=value)", path, lineno)
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		out[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
