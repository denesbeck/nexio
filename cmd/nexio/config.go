package main

import (
	"encoding/json"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(getCmd)

	setCmd.AddCommand(setDefaultBranchCmd)
	setCmd.AddCommand(setNameCmd)
	setCmd.AddCommand(setEmailCmd)

	getCmd.AddCommand(getDefaultBranchCmd)
	getCmd.AddCommand(getNameCmd)
	getCmd.AddCommand(getEmailCmd)
	getCmd.AddCommand(getUserCmd)
}

type Config struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set config values",
	Args:  cobra.ExactArgs(1),
}

var setDefaultBranchCmd = &cobra.Command{
	Use:     "default-branch",
	Short:   "Set default branch",
	Example: "nexio config set default-branch <branch-name>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Setting default branch: %s", args[0])
		setDefaultBranch(args[0])
	},
}

var setNameCmd = &cobra.Command{
	Use:     "name",
	Short:   "Set name",
	Example: "nexio config set name <name>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Setting name: %s", args[0])
		setConfig("name", args[0])
	},
}

var setEmailCmd = &cobra.Command{
	Use:     "email",
	Short:   "Set email",
	Example: "nexio config set email <email>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Setting email: %s", args[0])
		setConfig("email", args[0])
	},
}

var getCmd = &cobra.Command{
	Use:     "get",
	Short:   "Get config values",
	Example: "nexio config get name <name>",
	Args:    cobra.ExactArgs(1),
}

var getDefaultBranchCmd = &cobra.Command{
	Use:     "default-branch",
	Short:   "Get default branch",
	Example: "nexio config get default-branch",
	Args:    cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Getting default branch")
		getDefaultBranch()
	},
}

var getNameCmd = &cobra.Command{
	Use:     "name",
	Short:   "Get name",
	Example: "nexio config get name",
	Args:    cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Getting name")
		getConfig("name")
	},
}

var getEmailCmd = &cobra.Command{
	Use:     "email",
	Short:   "Get email",
	Example: "nexio config get email <email>",
	Args:    cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Getting email")
		getConfig("email")
	},
}

var getUserCmd = &cobra.Command{
	Use:     "user",
	Short:   "Get name and email",
	Example: "nexio config get user",
	Args:    cobra.ExactArgs(0),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Getting user info")
		getConfig("user")
	},
}

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "Config manager",
	Args:    cobra.ExactArgs(1),
}

func setConfig(key string, value string) int {
	Debug("Setting config: key=%s, value=%s", key, value)
	if initialized := IsInitialized(); !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001
	}
	config, err := os.ReadFile(GetDir("config"))
	if err != nil {
		Debug("Failed to read config file")
		MustSucceed(err, "operation failed")
	}

	var content Config
	if err = json.Unmarshal(config, &content); err != nil {
		Debug("Failed to unmarshal config")
		MustSucceed(err, "operation failed")
	}

	switch key {
	case "name":
		content.Name = value
	case "email":
		content.Email = value
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		Debug("Failed to marshal config")
		MustSucceed(err, "operation failed")
	}

	if err = os.WriteFile(GetDir("config"), jsonData, 0644); err != nil {
		Debug("Failed to write config file")
		MustSucceed(err, "operation failed")
	}
	Debug("Config updated successfully")
	Info("%s set to %s.", Capitalize(key), color.BlueString(value))
	return 603
}

func getConfig(key string) (returnCode int, conf Config) {
	Debug("Getting config: key=%s", key)
	if initialized := IsInitialized(); !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001, Config{}
	}
	config := GetConfig()
	switch key {
	case "name":
		if config.Name == "" {
			Debug("%s", CONFIG_RETURN_CODES[605])
			Fail("%s", CONFIG_RETURN_CODES[605])
			return 605, Config{}
		}
		Debug("Name: %s", config.Name)
		Info("%s: %s", Capitalize(key), color.BlueString(config.Name))
	case "email":
		if config.Email == "" {
			Debug("%s", CONFIG_RETURN_CODES[606])
			Fail("%s", CONFIG_RETURN_CODES[606])
			return 606, Config{}
		}
		Debug("Email: %s", config.Email)
		Info("%s: %s", Capitalize(key), color.BlueString(config.Email))
	case "user":
		if config.Name == "" || config.Email == "" {
			Debug("%s", CONFIG_RETURN_CODES[607])
			Fail("%s", CONFIG_RETURN_CODES[607])
			return 607, Config{}
		}
		Debug("User: %s <%s>", config.Name, config.Email)
		Info("%s: %s", Capitalize(key), color.BlueString(config.Name+" <"+config.Email+">"))
	}
	return 604, *config
}

func setDefaultBranch(branch string) int {
	if initialized := IsInitialized(); !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001
	}
	err := SetBranch(branch, "default")
	if err != nil {
		if err.Error() == BRANCH_RETURN_CODES[215] {
			Info("Default branch already set to %s.", StyledBranch(branch))
			return 215
		}
		if err.Error() == BRANCH_RETURN_CODES[216] {
			Fail("Branch does not exist: %s", StyledBranch(branch))
			return 216
		}
	}
	Debug("Default branch set successfully")
	Success("Default branch set to %s.", StyledBranch(branch))
	return 602
}

func getDefaultBranch() (returnCode int, defaultBranch string) {
	Debug("Getting default branch")
	if initialized := IsInitialized(); !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001, ""
	}
	branch := GetDefaultBranchName()
	Debug("Default branch: %s", branch)
	Info("Default branch: %s", StyledBranch(branch))
	return 601, branch
}
