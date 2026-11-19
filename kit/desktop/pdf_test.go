package desktop

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const pdfHelperEnv = "ZEN_SAVEPDF_HELPER"

// TestMain lets TestSavePDF run SavePDF in a child process: a Wails
// application needs the main thread, which a test function does not have.
func TestMain(m *testing.M) {
	if out := os.Getenv(pdfHelperEnv); out != "" {
		os.Exit(savePDFHelper(out))
	}
	os.Exit(m.Run())
}

// savePDFHelper runs a hidden accessory application with one page of text,
// writes it with SavePDF and prints "pages=<n>" or "error=<message>".
func savePDFHelper(out string) int {
	const page = `<!doctype html><html><head><title>t</title></head><body><h1>Hello</h1>%s</body></html>`
	body := strings.Repeat("<p>The quick brown fox jumps over the lazy dog. </p>", 400)
	a := application.New(application.Options{
		Name: "savepdf-test",
		Assets: application.AssetOptions{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, page, body)
		})},
		Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
	})
	w := a.Window.NewWithOptions(application.WebviewWindowOptions{Name: "t", URL: "/", Hidden: true, Width: 800, Height: 600})
	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			time.Sleep(1500 * time.Millisecond) // the page loads
			n, err := SavePDF(w, out, "Test")
			if err != nil {
				fmt.Println("error=" + err.Error())
			} else {
				fmt.Printf("pages=%d\n", n)
			}
			a.Quit()
		}()
	})
	if err := a.Run(); err != nil {
		fmt.Println("error=" + err.Error())
		return 1
	}
	return 0
}

func TestSavePDF(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if _, err := SavePDF(nil, "x.pdf", "x"); !errors.Is(err, ErrPDFUnsupported) {
			t.Fatalf("SavePDF off macOS = %v, want ErrPDFUnsupported", err)
		}
		return
	}
	if !HasWindowServer() {
		t.Skip("no window server (not a logged-in macOS session)")
	}
	out := filepath.Join(t.TempDir(), "t.pdf")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), pdfHelperEnv+"="+out)
	done := make(chan struct{})
	var got []byte
	var err error
	go func() { got, err = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("the helper did not finish in 60 s")
	}
	m := regexp.MustCompile(`pages=(\d+)`).FindStringSubmatch(string(got))
	if err != nil || m == nil {
		t.Fatalf("helper: %v\n%s", err, got)
	}
	pages, _ := strconv.Atoi(m[1])
	data, rerr := os.ReadFile(out)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.HasPrefix(string(data), "%PDF-") {
		t.Fatalf("not a PDF: %q", data[:min(len(data), 16)])
	}
	if pages < 2 {
		t.Errorf("pages = %d, want several for a long page", pages)
	}
	if n := countPDFPages(data); n != pages {
		t.Errorf("counted %d pages in the file, SavePDF said %d", n, pages)
	}
}

func TestCountPDFPages(t *testing.T) {
	pdf := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R >>\nendobj\n4 0 obj\n<< /Type/Page /Parent 2 0 R >>\nendobj\n"
	if n := countPDFPages([]byte(pdf)); n != 2 {
		t.Errorf("countPDFPages = %d, want 2 (not the /Pages node, not the catalog)", n)
	}
	if n := countPDFPages([]byte("nothing")); n != 0 {
		t.Errorf("countPDFPages(junk) = %d", n)
	}
}

// SavePDF refuses what it cannot write before any window is touched.
func TestSavePDFChecksTarget(t *testing.T) {
	if _, err := SavePDF(nil, "", "t"); err == nil {
		t.Error("an empty path is an error")
	}
	if _, err := SavePDF(nil, filepath.Join(t.TempDir(), "nosuch", "x.pdf"), "t"); err == nil {
		t.Error("a missing folder is an error")
	}
	if _, err := SavePDF(nil, t.TempDir(), "t"); err == nil {
		t.Error("a folder is not a file")
	}
}
