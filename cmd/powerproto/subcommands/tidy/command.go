// Copyright 2021 storyicon@foxmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tidy

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ppaanngggg/powerproto/pkg/bootstraps"
	"github.com/ppaanngggg/powerproto/pkg/component/pluginmanager"
	"github.com/ppaanngggg/powerproto/pkg/configs"
	"github.com/ppaanngggg/powerproto/pkg/consts"
	"github.com/ppaanngggg/powerproto/pkg/util"
	"github.com/ppaanngggg/powerproto/pkg/util/logger"
	"github.com/ppaanngggg/powerproto/pkg/util/progressbar"
)

func tidy(ctx context.Context,
	pluginManager pluginmanager.PluginManager,
	configFilePath string) error {
	progress := progressbar.GetProgressBar(ctx, 1)
	progress.SetPrefix("tidy config")
	err := bootstraps.StepTidyConfigFile(ctx, pluginManager, progress, configFilePath)
	if err != nil {
		return err
	}
	progress.Incr()
	progress.Wait()
	configItems, err := configs.LoadConfigItems(configFilePath)
	if err != nil {
		return err
	}
	if err := bootstraps.StepInstallProtoc(ctx, pluginManager, configItems); err != nil {
		return err
	}
	if err := bootstraps.StepInstallRepositories(ctx, pluginManager, configItems); err != nil {
		return err
	}
	if err := bootstraps.StepInstallPlugins(ctx, pluginManager, configItems); err != nil {
		return err
	}
	return nil
}

// CommandTidy is used to clean the config file
// By default, clean the powerproto.yaml of the current directory and all parent directories
// You can also explicitly specify the configuration file to clean up
func CommandTidy(log logger.Logger) *cobra.Command {
	var debugMode bool
	perCommandTimeout := time.Second * 300
	cmd := &cobra.Command{
		Use:   "tidy [config file]",
		Short: "tidy the config file. It will replace the version number and install the protoc and proto plugins that declared in the config file",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			ctx = consts.WithPerCommandTimeout(ctx, perCommandTimeout)
			log.SetLogLevel(logger.LevelInfo)
			if debugMode {
				log.SetLogLevel(logger.LevelDebug)
				ctx = consts.WithDebugMode(ctx)
				log.LogWarn(nil, "running in debug mode")
			}

			var targets []string
			if len(args) != 0 {
				for _, arg := range args {
					dir, _ := filepath.Abs(arg)
					targets = append(targets, dir)
				}
			} else {
				dir, err := os.Getwd()
				if err != nil {
					log.LogFatal(nil, "failed to get current dir: %s", err)
				}
				targets = configs.ListConfigPaths(dir)
			}
			pluginManager, err := pluginmanager.NewPluginManager(pluginmanager.NewConfig(), log)
			if err != nil {
				log.LogFatal(nil, "failed to create plugin manager: %s", err)
			}
			configMap := map[string]struct{}{}
			for _, path := range targets {
				exists, err := util.IsFileExists(path)
				if err != nil {
					log.LogFatal(map[string]interface{}{
						"path": path,
						"err":  err,
					}, "failed to tidy config")
				}
				if !exists {
					continue
				}
				log.LogInfo(nil, "tidy %s", path)
				if err := tidy(ctx, pluginManager, path); err != nil {
					log.LogFatal(map[string]interface{}{
						"path": path,
						"err":  err,
					}, "failed to tidy config")
				}
				configMap[path] = struct{}{}
			}
			log.LogInfo(nil, "these following config files were tidied:")
			for path := range configMap {
				log.LogInfo(nil, "	%s", path)
			}
			log.LogInfo(nil, "\r\nsucceeded, you are ready to go :)")
		},
	}
	flags := cmd.PersistentFlags()
	flags.BoolVarP(&debugMode, "debug", "d", debugMode, "debug mode")
	flags.DurationVarP(&perCommandTimeout, "timeout", "t", perCommandTimeout, "execution timeout for per command")
	return cmd
}
