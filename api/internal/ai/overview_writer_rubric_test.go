package ai

import (
	"strings"
	"testing"
)

// Rubrik penulis dan penilai harus berasal dari daftar indikator yang sama,
// supaya The Writer tidak bisa menulis untuk rubrik yang berbeda dari yang
// dipakai Gatekeeper saat menilai.
func TestAIOverviewWriterRubricMatchesGrader(t *testing.T) {
	st := DefaultAIOverviewSettings()
	joined := strings.Join(AIOverviewWriterRubric(st, "blogger"), "\n")
	for _, ind := range AIOverviewIndicators {
		if ind.WriterRule == "" {
			t.Fatalf("indikator %s tidak punya WriterRule", ind.Key)
		}
		if !strings.Contains(joined, ind.Title) {
			t.Fatalf("rubrik penulis kehilangan judul indikator %q", ind.Title)
		}
		if !strings.Contains(AIOverviewSystemPrompt, ind.Title) || !strings.Contains(AIOverviewSystemPrompt, ind.PromptKey) {
			t.Fatalf("prompt penilai kehilangan indikator %s/%s", ind.PromptKey, ind.Title)
		}
	}
	if len(AIOverviewIndicators) != len(aiOverviewCanonical) {
		t.Fatalf("canonical %d != indikator %d", len(aiOverviewCanonical), len(AIOverviewIndicators))
	}

	// Ambang diambil dari settings, bukan angka hardcode.
	custom := st
	custom.PassTotal = 36
	custom.PassMinIndicator = 8
	if got := strings.Join(AIOverviewWriterRubric(custom, "mainstream"), "\n"); !strings.Contains(got, "total minimal 36 dari 40 DAN setiap indikator minimal 8") {
		t.Fatalf("ambang settings tidak dipakai di rubrik penulis: %s", got)
	}
}

func TestAIOverviewSchemaFragmentShared(t *testing.T) {
	frag := AIOverviewIndicatorSchemaFragment()
	for _, ind := range AIOverviewIndicators {
		if !strings.Contains(frag, `"kunci":"`+ind.PromptKey+`"`) {
			t.Fatalf("fragment skema kehilangan %s", ind.PromptKey)
		}
	}
	if !strings.Contains(AIOverviewSystemPrompt, frag) {
		t.Fatal("prompt AI Overview tidak memakai fragment skema bersama")
	}
	if !strings.Contains(GuardSystemPrompt("aturan domain"), frag) {
		t.Fatal("prompt gabungan guard tidak memakai fragment skema bersama")
	}
}
