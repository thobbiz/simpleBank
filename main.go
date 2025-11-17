package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hibiken/asynq"
	_ "github.com/lib/pq"
	"github.com/rakyll/statik/fs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/thobbiz/simplebank/api"
	db "github.com/thobbiz/simplebank/db/sqlc"
	_ "github.com/thobbiz/simplebank/doc/statik"
	"github.com/thobbiz/simplebank/gapi"
	"github.com/thobbiz/simplebank/pb"
	"github.com/thobbiz/simplebank/util"
	"github.com/thobbiz/simplebank/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	logLogger := log.Logger
	logger := logLogger.Info()

	config, err := util.LoadConfig(".")
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot load config:")
	}

	if config.Environment == "development" {
		logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot connect to db!")
	}

	runDBMigration(config.MigrationURL, config.DBSource)

	store := db.NewStore(conn)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	go runTaskProcessor(redisOpt, store)
	go runGatewayServer(config, store, taskDistributor)
	runGrpcServer(config, store, taskDistributor)
}

func runDBMigration(migrationURL string, dbSource string) {
	logLogger := log.Logger
	logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := logLogger.Info()

	migration, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create new migrate instance: ")
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("failed to run migrate up: ")
	}

	logger.Msg("db migrated successfully")
}

func runTaskProcessor(redisOpt asynq.RedisClientOpt, store db.Store) {
	logLogger := log.Logger
	logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := logLogger.Info()

	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store)
	logger.Msg("start task processor")

	err := taskProcessor.Start()
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("failed to start task processor")
	}
}

func runGrpcServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	logLogger := log.Logger
	logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := logLogger.Info()

	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create server: ")
	}

	grpcLogger := grpc.UnaryInterceptor(gapi.GrpcLogger)
	grpcServer := grpc.NewServer(grpcLogger)

	pb.RegisterSimpleBankServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", config.GRPCServerAddress)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create listener")
	}

	logger.Msgf("start gRPC server at %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot start gRPC server")
	}
}

func runGatewayServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	logLogger := log.Logger
	logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := logLogger.Info()

	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create server: ")
	}

	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOption)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot register handler server: ")
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	statikFS, err := fs.New()
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create statik fs: ")
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	listener, err := net.Listen("tcp", config.HTTPServerAddress)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create listener: ")
	}

	logger.Msgf("start HTTP gateway server at %s", listener.Addr().String())
	handler := gapi.HttpLogger(mux)
	err = http.Serve(listener, handler)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot start HTTP gateway server: ")
	}
}

func runGinServer(config util.Config, store db.Store) {
	logLogger := log.Logger
	logLogger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logger := logLogger.Info()

	server, err := api.NewServer(config, store)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("cannot create server: ")
	}

	err = server.Start(config.HTTPServerAddress)
	if err != nil {
		logger = logLogger.Fatal().Err(err)
		logger.Msg("Cannot start server!")
	}
}
