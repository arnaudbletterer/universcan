package diagnostic

import (
	"context"
	"testing"
	"time"
)

func TestDiagnosticCollector(t *testing.T) {
	collector := NewCollector()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	report, err := collector.GenerateReport(ctx, "192.168.1.50")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}

	t.Logf("Manufacturer: %s, Model: %s, Recommended: %s",
		report.Manufacturer, report.Model, report.RecommendedProtocol)
	t.Logf("Responsive ports: %v", report.ResponsivePorts)
	t.Logf("Markdown Summary:\n%s", report.MarkdownSummary)
}
