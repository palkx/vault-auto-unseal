package command

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miknikif/vault-auto-unseal/common"
	"github.com/miknikif/vault-auto-unseal/keys"
	"github.com/miknikif/vault-auto-unseal/policies"
	"github.com/miknikif/vault-auto-unseal/sys"
	"github.com/miknikif/vault-auto-unseal/tokens"
)

// Seed DB
func Seed(c *common.Config) error {
	c.Logger.Info("Seeding DB")
	if err := policies.SeedDB(c); err != nil {
		return err
	}
	if err := tokens.SeedDB(c); err != nil {
		return err
	}
	c.Logger.Info("Seeding completed")
	return nil
}

// Migrate provided DB
func Migrate(c *common.Config) error {
	c.Logger.Info(fmt.Sprintf("Migrating %s", c.Args.DBName))
	c.DB.AutoMigrate(&policies.PolicyModel{})
	c.DB.AutoMigrate(&tokens.TokenModel{})
	c.DB.AutoMigrate(&keys.AESKeyModel{})
	c.DB.AutoMigrate(&keys.KeyModel{})
	c.Logger.Info(fmt.Sprintf("Migration of the %s DB completed", c.Args.DBName))
	if c.DBStatus == common.INIT_DB_RES_CREATED {
		if err := Seed(c); err != nil {
			return err
		}
	}
	return nil
}

// Start HTTP Server
func StartHttpServer() error {
	c, err := common.GetConfig()
	if err != nil {
		return err
	}
	if err := Migrate(c); err != nil {
		return err
	}
	defer c.DB.Close()

	if c.Args.IsProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.Use(common.JSONMiddleware(false))
	router.Use(common.RequestIDMiddleware())
	sys.HealthRegister(router.Group("/v1/sys"))

	v1 := router.Group("/v1")
	v1.Use(tokens.AuthMiddleware())
	tokens.TokenRegister(v1.Group("/auth/token"))
	policies.PolicyRegister(v1.Group("/sys/policy"))
	policies.PolicyRegister(v1.Group("/sys/policies/acl"))
	keys.KeysOperationsRegister(v1.Group("/transit"))

	server := &http.Server{
		Addr:     fmt.Sprintf("%s:%d", c.Args.Host, c.Args.Port),
		Handler:  router,
		ErrorLog: c.Logger.StandardLogger(nil),
	}

	if c.TLS != nil {
		server.TLSConfig = c.TLS.TLSConfig
		fmt.Println(c.TLS.BundleCrt)
		c.Logger.Info(fmt.Sprintf("Starting HTTPS server at https://%s:%d", c.Args.Host, c.Args.Port))
		err = server.ListenAndServeTLS(c.TLS.BundleCrt, c.TLS.TLSKey)
		if err != nil {
			return err
		}
	} else {
		c.Logger.Info(fmt.Sprintf("Starting HTTP server at http://%s:%d", c.Args.Host, c.Args.Port))
		err = server.ListenAndServe()
		if err != nil {
			return err
		}
	}
	return nil
}
