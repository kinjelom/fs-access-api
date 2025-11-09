package app

import (
	"fmt"
	"fs-access-api/internal/adapters/in/rest"
	"fs-access-api/internal/adapters/in/rest/openapi"
	"fs-access-api/internal/adapters/out/accounts"
	"fs-access-api/internal/adapters/out/fs"
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/api"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/docs"
	"fs-access-api/internal/app/ports"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func BuildApiServer(cfg *config.ProgramConfig, bootstrap bool) (ports.ApiServer, error) {
	hasher, err := security.NewDefaultHasherFromConfig(cfg.Security.Hasher)
	if err != nil {
		return nil, fmt.Errorf("cannot create hasher: %v", err)
	}

	accountRepo, err := createAccountRepo(cfg, bootstrap)
	if err != nil {
		return nil, err
	}

	fsService, err := CreateFilesystemService(cfg.Storage.Implementation)
	if err != nil {
		return nil, fmt.Errorf("cannot create filesytem service: %v", err)
	}

	fsStorageService, err := fs.NewDefaultFsStorageService(cfg.Storage, fsService, bootstrap)
	if err != nil {
		return nil, fmt.Errorf("cannot create filesytem service: %v", err)
	}

	apiServer, err := api.NewDefaultApiServer(cfg.Storage, hasher, accountRepo, fsStorageService)
	if err != nil {
		return nil, fmt.Errorf("cannot create api server: %v", err)
	}

	if bootstrap && cfg.AccountRepository.LoadInitialData {
		err = loadInitialData(apiServer, cfg)
		if err != nil {
			return nil, fmt.Errorf("cannot load initial data: %v", err)
		}
	}
	return apiServer, nil
}

func CreateFilesystemService(implementation string) (ports.FilesystemService, error) {
	switch implementation {
	case "none":
		return fs.NewNoneFilesystemService(), nil
	case "inmem":
		return fs.NewInMemFilesystemService(), nil
	case "unix":
		return fs.NewUnixFilesystemService(), nil
	default:
		return nil, fmt.Errorf("unsupported filesystem implementation: '%s'", implementation)
	}
}

func BuildRestServer(cfg *config.ProgramConfig, bootstrap bool, actionMetrics ports.ActionMetrics) (*rest.DefaultRestServer, error) {
	apiServer, err := BuildApiServer(cfg, bootstrap)
	if err != nil {
		return nil, fmt.Errorf("cannot create api server: %v", err)
	}

	authenticator, err := security.NewMultiAuthenticator(cfg.Security.Authenticator)
	if err != nil {
		return nil, fmt.Errorf("cannot create Authenticator: %v", err)
	}

	restServer, err := rest.NewRestServer(cfg.HttpServer, apiServer, authenticator, actionMetrics)
	if err != nil {
		return nil, fmt.Errorf("cannot create rest server: %v", err)
	}
	return restServer, nil
}

func createAccountRepo(cfg *config.ProgramConfig, bootstrap bool) (accountRepo ports.AccountRepository, err error) {
	switch cfg.AccountRepository.Type {
	case "inmem":
		accountRepo, err = accounts.NewInMemAccountRepository(cfg.AccountRepository.InMem, cfg.AccountRepository.Common, bootstrap)
		break
	case "sqlite":
		accountRepo, err = accounts.NewSQLiteAccountRepository(cfg.AccountRepository.Sqlite, cfg.AccountRepository.Common, bootstrap)
		break
	case "mysql":
		accountRepo, err = accounts.NewMySQLAccountRepository(cfg.AccountRepository.MySQL, cfg.AccountRepository.Common, bootstrap)
		break
	default:
		return nil, fmt.Errorf("unsupported account repository type: %s", cfg.AccountRepository.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create account repository with type '%s': %v", cfg.AccountRepository.Type, err)
	}
	info, err := accountRepo.GetInfo()
	if err == nil {
		log.Printf("Account repository info: %s", info)
	} else {
		return nil, fmt.Errorf("failed to get account repository ('%s') info: %v", cfg.AccountRepository.Type, err)
	}
	return accountRepo, nil
}

func loadInitialData(apiServer ports.ApiServer, cfg *config.ProgramConfig) (err error) {
	log.Printf("Loading initial data...")
	icr := 0
	iex := 0
	ier := 0
	for name, entityInfo := range cfg.GetInitialGroups() {
		var created bool
		_, created, err = apiServer.EnsureGroup(*entityInfo)
		if err != nil {
			log.Printf("Group '%s' can't be ensured, error: %v", name, err)
			ier++
		}
		if created {
			log.Printf("Group '%s' created, home: %s", name, entityInfo.AbsoluteHomeDir(cfg.Storage.HomesBaseDir))
			icr++
		} else {
			log.Printf("Group '%s' already existed, home: %s", name, entityInfo.AbsoluteHomeDir(cfg.Storage.HomesBaseDir))
			iex++
		}
	}
	log.Printf("Groups existed %d, loaded %d, errored: %d", iex, icr, ier)

	icr = 0
	iex = 0
	ier = 0
	for name, entityInfo := range cfg.GetInitialUsers() {
		g, err := apiServer.GetGroup(entityInfo.Groupname)
		if err != nil {
			log.Printf("User '%s' can't be ensured because can't read his group, error: %v", name, err)
			ier++
		}
		_, err = apiServer.GetUser(name)
		var created bool
		_, created, err = apiServer.EnsureUser(*entityInfo)
		if err != nil {
			log.Printf("User '%s' can't be ensured, error: %v", name, err)
			ier++
		}
		if created {
			log.Printf("User '%s' created, home: %s", name, entityInfo.AbsoluteHomeDir(cfg.Storage.HomesBaseDir, g.Home))
			icr++
		} else {
			log.Printf("User '%s' already existed, home: %s", name, entityInfo.AbsoluteHomeDir(cfg.Storage.HomesBaseDir, g.Home))
			iex++
		}
	}
	log.Printf("Users existed %d, loaded %d, errored: %d", iex, icr, ier)
	return nil
}

func BuildRouter(server openapi.ServerInterface) *chi.Mux {
	// Router CHI
	r := chi.NewRouter()

	// Standard middlewares: request correlation, real client IP, logging, recovery, and server-side request timeout
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	_ = openapi.HandlerFromMux(server, r)

	// Health and readiness probes
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Basic readiness check; extend with deeper dependencies if needed
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// Index page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(docs.IndexHTML)
	})
	// ReDoc UI
	r.Get("/docs/redoc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(docs.RedocHTML)
	})
	// Swagger UI
	r.Get("/docs/swagger", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(docs.SwaggerHTML)
	})

	// OpenAPI YAML
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(docs.OpenAPIYAML)
	})
	return r
}
