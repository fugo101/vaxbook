// PROTOTYPE — throwaway. Builds dist/vaxbook-engine-prototype.html:
// compiles the engine to js/wasm, measures size, runs the scenarios natively (golden output + timing)
// and inlines everything into one double-clickable HTML file.
//
//	cd engine-prototype && go run ./cmd/build
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"vaxbook.local/engine-prototype/engine"
	"vaxbook.local/engine-prototype/scenarios"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func goEnv(k string) string {
	out, err := exec.Command("go", "env", k).Output()
	must(err)
	return strings.TrimSpace(string(out))
}

func main() {
	must(os.MkdirAll("dist", 0o755))

	// 1. WASM build.
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", "dist/engine.wasm", "./cmd/wasm")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	must(cmd.Run())
	wasm, err := os.ReadFile("dist/engine.wasm")
	must(err)

	var gz bytes.Buffer
	w, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	w.Write(wasm)
	w.Close()
	brSize := -1
	if br, err := exec.Command("brotli", "-q", "11", "-c", "dist/engine.wasm").Output(); err == nil {
		brSize = len(br)
	}
	wasmExec, err := os.ReadFile(filepath.Join(goEnv("GOROOT"), "lib", "wasm", "wasm_exec.js"))
	must(err)

	// 2. Native golden run — same path as WASM: JSON in → Evaluate → JSON out.
	var rs engine.RuleSet
	must(json.Unmarshal(scenarios.RuleSetJSON, &rs))
	eng, err := engine.New(&rs)
	must(err)
	scs := scenarios.All()
	golden := map[string][]string{}
	var inputs [][]byte
	for _, sc := range scs {
		for i, st := range sc.Steps {
			raw, _ := json.Marshal(st.Input)
			inputs = append(inputs, raw)
			var in engine.Input
			must(json.Unmarshal(raw, &in))
			out, _ := json.Marshal(eng.Evaluate(in))
			golden[sc.ID] = append(golden[sc.ID], string(out))
			fmt.Printf("\n[%s #%d] %s\n", sc.ID, i+1, st.Label)
			var o engine.Output
			_ = json.Unmarshal(out, &o)
			for _, a := range o.Assessments {
				var codes []string
				for _, r := range a.Reasons {
					codes = append(codes, r.Code+fmtParams(r.Params))
				}
				nd := ""
				if a.NextDose != nil && a.NextDose.Due != nil {
					nd = fmt.Sprintf(" next#%d earliest=%s due=%s overdue=%s", a.NextDose.Number, a.NextDose.Earliest, a.NextDose.Due, a.NextDose.Overdue)
				}
				fmt.Printf("   %-10s %-9s%s%s est=%v %v\n", a.Disease, a.Bucket, a.Block, nd, a.Estimated, codes)
			}
		}
	}

	// 3. Native timing.
	const iters = 2000
	t0 := time.Now()
	for i := 0; i < iters; i++ {
		var in engine.Input
		raw := inputs[i%len(inputs)]
		_ = json.Unmarshal(raw, &in)
		out := eng.Evaluate(in)
		_, _ = json.Marshal(out)
	}
	nativeUs := float64(time.Since(t0).Microseconds()) / iters

	metrics := map[string]any{
		"goVersion": goEnv("GOVERSION"), "wasmRaw": len(wasm), "wasmGzip": gz.Len(), "wasmBrotli": brSize,
		"nativeMicrosPerEval": nativeUs, "builtAt": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(map[string]any{
		"ruleSetJSON": string(scenarios.RuleSetJSON), "scenarios": scs, "golden": golden, "metrics": metrics,
	})
	goldenJSON, _ := json.MarshalIndent(golden, "", "  ")
	must(os.WriteFile("dist/golden.json", goldenJSON, 0o644))

	tpl, err := os.ReadFile("web/template.html")
	must(err)
	html := string(tpl)
	html = strings.Replace(html, "__WASM_EXEC_JS__", string(wasmExec), 1)
	html = strings.Replace(html, "__DATA_JSON__", strings.ReplaceAll(string(data), "</", `<\/`), 1)
	html = strings.Replace(html, "__WASM_B64__", base64.StdEncoding.EncodeToString(wasm), 1)
	must(os.WriteFile("dist/vaxbook-engine-prototype.html", []byte(html), 0o644))

	fmt.Printf("\nGo %s | wasm raw %d B, gzip %d B, brotli %d B | native %.1f µs/eval | html %d B\n",
		metrics["goVersion"], len(wasm), gz.Len(), brSize, nativeUs, len(html))
	abs, _ := filepath.Abs("dist/vaxbook-engine-prototype.html")
	fmt.Println("open:", abs)
}

func fmtParams(p map[string]string) string {
	if len(p) == 0 {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}
