/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"

	"github.com/paulfarver/valet/internal/github"
	"github.com/paulfarver/valet/internal/rest"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/uniwise/fxrus"
	"go.uber.org/fx"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: serve,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func serve(cmd *cobra.Command, args []string) {
	conf, err := loadConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	logger := GetLogger(conf.Log)

	app := fx.New(
		fx.WithLogger(fxrus.NewLogger(logger.WithField("entrypoint", "serve"))),
		fx.Supply(
			logger,
			conf.Rest,
			conf.Github,
		),

		fx.Provide(
			rest.NewServer,
			github.NewService,
		),

		fx.Invoke(serverLifecycle),
	)

	app.Run()
}

func serverLifecycle(lifecycle fx.Lifecycle, s fx.Shutdowner, l *logrus.Logger, server *rest.Server) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.Start(); err != nil {
					l.WithError(err).Error("failed to start server")
					if err := s.Shutdown(); err != nil {
						l.WithError(err).Fatal("failed to shutdown server")
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
