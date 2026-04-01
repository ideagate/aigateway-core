package db

import (
	"strings"
	"testing"

	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
)

func TestOpen_UnsupportedType(t *testing.T) {
	_, err := Open("sqlite", platformconfig.SQLConfig{})
	if err == nil {
		t.Fatal("Open() error = nil, want non-nil")
	}
}

func TestOpen_MySQLMissingHost(t *testing.T) {
	cfg := platformconfig.SQLConfig{Port: 3306, Username: "root", DBName: "aigateway"}

	_, err := Open(DBTypeMySQL, cfg)
	if err == nil {
		t.Fatal("Open() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "datastores.mysql.host is required") {
		t.Fatalf("Open() error = %q, want mysql host validation error", err.Error())
	}
}

func TestBuildMySQLDSN_DefaultParams(t *testing.T) {
	cfg := platformconfig.SQLConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "secret",
		DBName:   "aigateway",
	}

	dsn := buildMySQLDSN(cfg)
	if dsn != "root:secret@tcp(localhost:3306)/aigateway?parseTime=true" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestBuildMySQLDSN_PrefixesParams(t *testing.T) {
	cfg := platformconfig.SQLConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "secret",
		DBName:   "aigateway",
		Params:   "charset=utf8mb4",
	}

	dsn := buildMySQLDSN(cfg)
	if dsn != "root:secret@tcp(localhost:3306)/aigateway?charset=utf8mb4" {
		t.Fatalf("dsn = %q", dsn)
	}
}
