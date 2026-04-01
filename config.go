package aigatewaycore

type Config struct {
	Providers  *ProvidersConfig
	DataStores *DataStoresConfig
}

type ProvidersConfig struct {
	Google *GoogleProviderConfig
}

type GoogleProviderConfig struct {
	ApiKey string
}

type DataStoresConfig struct {
	Mysql    *MySQLConfig
	Postgres *PostgresConfig
	Redis    *RedisConfig
}

type MySQLConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	Database int
}
