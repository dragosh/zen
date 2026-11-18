// Command plist renders build/darwin/Info.plist: the template's {{NAME}}
// placeholders are filled from the version, the build number and the info:
// block of build/config.yml, so the product name, bundle id, copyright and
// category live in one place.
//
//	go run ./build/darwin/plist -config build/config.yml -template build/darwin/Info.plist -version 0.2.0 -build 123 -out bin/Zenmark.app/Contents/Info.plist
package main

import (
	"flag"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type config struct {
	Info struct {
		ProductName string `yaml:"productName"`
		BundleID    string `yaml:"bundleId"`
		Copyright   string `yaml:"copyright"`
		Category    string `yaml:"category"`
	} `yaml:"info"`
}

var placeholder = regexp.MustCompile(`\{\{([A-Z_]+)\}\}`)

// render fills the placeholders of tpl with XML-escaped values. An unknown
// placeholder is an error.
func render(tpl string, vals map[string]string) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(tpl, func(m string) string {
		name := placeholder.FindStringSubmatch(m)[1]
		v, ok := vals[name]
		if !ok {
			missing = append(missing, name)
			return m
		}
		return html.EscapeString(v)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("unknown placeholder %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func main() {
	cfgPath := flag.String("config", "build/config.yml", "build config")
	tplPath := flag.String("template", "build/darwin/Info.plist", "Info.plist template")
	version := flag.String("version", "", "CFBundleShortVersionString")
	build := flag.String("build", "", "CFBundleVersion")
	outPath := flag.String("out", "", "output file")
	flag.Parse()
	if err := run(*cfgPath, *tplPath, *version, *build, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "plist: %v\n", err)
		os.Exit(1)
	}
}

func run(cfgPath, tplPath, version, build, outPath string) error {
	if version == "" || build == "" || outPath == "" {
		return fmt.Errorf("-version, -build and -out are required")
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var cfg config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("%s: %w", cfgPath, err)
	}
	vals := map[string]string{
		"SHORT_VERSION": version,
		"BUILD_NUMBER":  build,
		"PRODUCT_NAME":  cfg.Info.ProductName,
		"BUNDLE_ID":     cfg.Info.BundleID,
		"COPYRIGHT":     cfg.Info.Copyright,
		"CATEGORY":      cfg.Info.Category,
	}
	for k, v := range vals {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s: info.%s is empty (needed for %s)", cfgPath, configKey[k], k)
		}
	}
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		return err
	}
	out, err := render(string(tpl), vals)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(out), 0o644)
}

var configKey = map[string]string{
	"SHORT_VERSION": "version", "BUILD_NUMBER": "build", "PRODUCT_NAME": "productName",
	"BUNDLE_ID": "bundleId", "COPYRIGHT": "copyright", "CATEGORY": "category",
}
