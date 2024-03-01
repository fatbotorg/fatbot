module fatbot

go 1.22

require (
	github.com/aws/aws-sdk-go v1.44.298
	github.com/charmbracelet/log v0.2.1
	github.com/getsentry/sentry-go v0.21.0
	github.com/go-co-op/gocron v1.25.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/gomodule/redigo v1.8.9
	github.com/henomis/quickchart-go v1.0.0
	github.com/sashabaranov/go-openai v1.14.0
	github.com/spf13/viper v1.16.0
	gorm.io/driver/sqlite v1.5.0
	gorm.io/gorm v1.25.0
)

// replace github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1 => /Users/omerxx/omerxx/telegram-bot-api

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/lipgloss v0.7.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mattn/go-sqlite3 v1.14.16 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/mattn/go-sqlite3 => github.com/leso-kn/go-sqlite3 v0.0.0-20230710125852-03158dc838ed

replace github.com/go-telegram-bot-api/telegram-bot-api/v5 => ./telegram-bot-api/
