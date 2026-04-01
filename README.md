# aigateway-core

# How to install
```bash
go get -u github.com/aigateway/aigateway-core
```

# Quick start

## Initialize the AI Gateway client with configuration
```go
package main

import (
    "github.com/aigateway/aigateway-core"
)

func main() {
    // Create a new AI Gateway client
    config := &aigateway.Config{
        Providers: &aigateway.ProvidersConfig{
            Google: &aigateway.GoogleProviderConfig{
                ApiKey: "your-google-api-key",
            },
        },
        DataStores: &aigateway.DataStoresConfig{
            Mysql:    &aigateway.MySQLConfig{    
                Host:     "localhost",
                Port:     3306,
                User:     "your-db-user",
                Password: "your-db-password",
                Database:   "your-db-name",
            },
            Postgres: &aigateway.PostgresConfig{
                Host:     "localhost",
                Port:     5432,
                User:     "your-db-user",
                Password: "your-db-password",
                Database:   "your-db-name",
            },
            Redis: &aigateway.RedisConfig{
                Addr:     "localhost",
				Port:     6379,
                Password: "your-redis-password",
                Database: 0,
            },
        },
        Schedulers: &aigateway.SchedulersConfig{
            SyncBatchJobStatus: &aigateway.SyncBatchJobStatusConfig{
                Interval: 5 * time.Minute,
            },
        },
    }
    
    client, err := aigateway.NewClient(config)
    if err != nil {
        panic(err)
    }
```