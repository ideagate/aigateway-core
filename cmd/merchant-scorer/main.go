// Command merchant-scorer demonstrates batch merchant description scoring
// using Google Gemini LLM via the google.golang.org/genai SDK.
//
// Usage:
//
//	# Use config/default.yaml (or pass -config)
//	go run ./cmd/merchant-scorer
//
//	# Vertex AI (can come from config or env override)
//	export GOOGLE_CLOUD_PROJECT=your_project
//	go run ./cmd/merchant-scorer -backend=vertexai
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	"google.golang.org/genai"

	"github.com/ideagate/aigateway-core/internal/scorer"
)

func main() {
	backend := flag.String("backend", "gemini", "API backend to use: gemini or vertexai")
	model := flag.String("model", scorer.DefaultModel, "Gemini model name to use for scoring")
	configPath := flag.String("config", "config/default.yaml", "path to YAML config file")
	flag.Parse()

	ctx := context.Background()
	cfg, err := platformconfig.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	clientCfg, err := buildClientConfig(*backend, cfg)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	s, err := scorer.New(ctx, clientCfg)
	if err != nil {
		log.Fatalf("create scorer: %v", err)
	}
	if *model != scorer.DefaultModel {
		s = s.WithModel(*model)
	}

	merchants := sampleMerchants()

	fmt.Printf("Submitting batch scoring job for %d merchant descriptions...\n\n", len(merchants))

	results, err := s.ScoreBatch(ctx, merchants)
	if err != nil {
		log.Fatalf("batch scoring failed: %v", err)
	}

	printResults(results)
}

// buildClientConfig creates a genai.ClientConfig for the chosen backend.
func buildClientConfig(backend string, cfg *platformconfig.Config) (*genai.ClientConfig, error) {
	switch backend {
	case "gemini":
		apiKey := cfg.Providers.Gemini.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("providers.gemini.api_key is not set")
		}
		return &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI}, nil
	case "vertexai":
		project := cfg.Providers.VertexAI.Project
		location := cfg.Providers.VertexAI.Location
		if project == "" {
			return nil, fmt.Errorf("providers.vertexai.project is not set")
		}
		if location == "" {
			location = "us-central1"
		}
		return &genai.ClientConfig{
			Project:  project,
			Location: location,
			Backend:  genai.BackendVertexAI,
		}, nil
	default:
		return nil, fmt.Errorf("unknown backend %q: choose 'gemini' or 'vertexai'", backend)
	}
}

// sampleMerchants returns a representative set of merchant descriptions for scoring.
func sampleMerchants() []scorer.MerchantInput {
	return []scorer.MerchantInput{
		{
			ID:          "merchant_001",
			Description: "We sell stuff. Open daily.",
		},
		{
			ID: "merchant_002",
			Description: "Artisan Brew Co. is a specialty coffee roastery and café located in " +
				"downtown Portland. We source single-origin beans directly from farmers in " +
				"Ethiopia, Colombia, and Guatemala, roast them in small batches, and serve " +
				"expertly crafted espresso drinks, pour-overs, and cold brew. Our cozy space " +
				"features free Wi-Fi and is perfect for remote work or catching up with friends.",
		},
		{
			ID: "merchant_003",
			Description: "Family-owned Italian restaurant serving authentic homemade pasta, " +
				"wood-fired pizza, and traditional desserts. Established in 1987. " +
				"Reservations recommended on weekends.",
		},
		{
			ID: "merchant_004",
			Description: "Best store in town!!! We have everything you need at the lowest " +
				"prices GUARANTEED. Come visit us today!!",
		},
		{
			ID: "merchant_005",
			Description: "GreenLeaf Organic Market offers a curated selection of certified " +
				"organic produce, bulk grains, and eco-friendly household products. " +
				"We partner with over 30 local farms to bring you the freshest seasonal " +
				"ingredients. Our knowledgeable staff can help you find allergen-free and " +
				"specialty diet options. Open 7 days a week, 8 AM–9 PM.",
		},
	}
}

// printResults displays the scoring results in a human-readable format.
func printResults(results []scorer.ScoreResult) {
	fmt.Println("=== Merchant Description Scores ===")
	for i, r := range results {
		fmt.Printf("\n[%d] Merchant ID: %s\n", i+1, r.MerchantID)
		fmt.Printf("    Description: %.80s...\n", r.Description)
		if r.Error != "" {
			fmt.Printf("    Error: %s\n", r.Error)
			continue
		}
		fmt.Printf("    Score:  %d / 10\n", r.Score)
		fmt.Printf("    Reason: %s\n", r.Reason)
	}

	fmt.Println("\n=== JSON Output ===")
	out, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("marshal results: %v", err)
		return
	}
	fmt.Println(string(out))
}
