package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	aigatewaycore "github.com/ideagate/aigateway-core"
	"github.com/ideagate/aigateway-core/models"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
)

func constructConfig() *aigatewaycore.Config {
	return &aigatewaycore.Config{
		Providers: &aigatewaycore.ProvidersConfig{
			Google: &aigatewaycore.GoogleProviderConfig{
				ApiKey: os.Getenv("GOOGLE_API_KEY"),
			},
		},
		DataStores: &aigatewaycore.DataStoresConfig{
			Mysql: &aigatewaycore.MySQLConfig{
				Host:     os.Getenv("MYSQL_HOST"),
				Port:     cast.ToInt(os.Getenv("MYSQL_PORT")),
				User:     os.Getenv("MYSQL_USER"),
				Password: os.Getenv("MYSQL_PASSWORD"),
				Database: os.Getenv("MYSQL_DATABASE"),
			},
			Redis: &aigatewaycore.RedisConfig{
				Host:     os.Getenv("REDIS_HOST"),
				Port:     cast.ToInt(os.Getenv("REDIS_PORT")),
				Password: os.Getenv("REDIS_PASSWORD"),
				Database: cast.ToInt(os.Getenv("REDIS_DATABASE")),
			},
		},
	}
}

func TestMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := constructConfig()
	client, err := aigatewaycore.NewClient(config)
	if err != nil {
		assert.NoErrorf(t, err, "failed to initialize client: %v", err)
		return
	}

	if err = client.Migrate(t.Context()); err != nil {
		assert.NoErrorf(t, err, "migration failed: %v", err)
		return
	}

	if err = client.CheckMigration(t.Context()); err != nil {
		assert.NoErrorf(t, err, "migration failed: %v", err)
		return
	}
}

func TestChatCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := constructConfig()
	client, err := aigatewaycore.NewClient(config)
	if err != nil {
		assert.NoErrorf(t, err, "failed to initialize client: %v", err)
		return
	}

	request := &models.ChatCompletionRequest{
		Model: "gemini-flash-lite-latest",
		Content: models.Content{
			Text: "Warung kami menyajikan jenis kopi robusta yang berasal dari daerah kami sendiri,yang diseduh secara manual ,kami juga menyajikan sedikit sarapan pagi seperti mie gomak kuah santan , dan kami juga menyajikan beberapa kopi sachet yang dibungkus menggunakan kemasan untuk kedai kopi kami",
		},
		SystemInstruction: models.Content{
			Text: "This instruction is using Bahasa Indonesia Language. \n\nAnda adalah seorang pakar untuk menilai skor dari sebuah deskripsi proposal untuk bantuan usaha. Aspek-aspek yang harus dinilai adalah:\n\n- Deskripsi Produk/Jasa Usaha yang Dihasilkan\n- Alasan mengapa memilih usaha yang diajukan\n- Gambaran Potensi Usaha kedepannya\n\nDeskripsi usaha tersebut harus berkesinambungan diantara ketiga poin tersebut.\nUntuk rentang skor berada di 0 (sangat buruk) hingga 100 (sangat baik). Buat penjelasan tentang skor tidak lebih dari 1000 huruf dan buat dalam penjelasan per poin",
		},
		JsonSchemaResponse: "{\"type\":\"object\",\"properties\":{\"average_score\":{\"type\":\"number\"},\"breakdowns\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"title\":{\"type\":\"string\"},\"score\":{\"type\":\"number\"},\"explanation\":{\"type\":\"string\"}},\"required\":[\"title\",\"score\",\"explanation\"]}}},\"required\":[\"average_score\",\"breakdowns\"]}",
	}

	result, err := client.ChatCompletion(t.Context(), request)
	if err != nil {
		assert.NoErrorf(t, err, "chat completion failed: %v", err)
		return
	}

	resultText := result.Content.Text
	assert.NotEmptyf(t, resultText, "chat completion response is empty")

	// format result based on JsonSchemaResponse
	resultFormatted := struct {
		AverageScore float64 `json:"average_score"`
		Breakdowns   []struct {
			Title       string  `json:"title"`
			Score       float64 `json:"score"`
			Explanation string  `json:"explanation"`
		} `json:"breakdowns"`
	}{}
	assert.NoErrorf(t, json.Unmarshal([]byte(resultText), &resultFormatted), "failed to unmarshal chat completion response")
	fmt.Printf("result text formatted: %+v\n", resultFormatted)
}
