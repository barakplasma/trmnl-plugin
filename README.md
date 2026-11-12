# TRMNL Plugins

Plugins for [TRMNL](https://usetrmnl.com) e-ink displays, written for a self-hosted
[BYOS Laravel](https://github.com/usetrmnl/larapaper) server. Each plugin is a `settings.yml` plus Liquid templates
that you import as a ZIP.

| Plugin | What it shows | Needs a server? | Latest release |
|---|---|---|---|
| [Kids Age](#kids-age) | Your kids' ages, computed daily from their birthdays | No, runs entirely in Liquid | [`0.0.2`](https://github.com/barakplasma/trmnl-plugin/releases/tag/0.0.2) |
| [Israel Bus](israel-bus/) | Upcoming Israeli bus and light rail departures, in Hebrew, right to left | Yes, a small Go service | [`israel-bus-v0.2.0`](https://github.com/barakplasma/trmnl-plugin/releases/tag/israel-bus-v0.2.0) |

## Install a plugin

1. Download the plugin ZIP from the [latest release](https://github.com/barakplasma/trmnl-plugin/releases) of the plugin you want
   (`kids-age-plugin.zip` or `israel-bus-plugin.zip`).
2. In your TRMNL instance, import the ZIP as a new plugin.
3. Fill in the plugin settings.

Israel Bus also needs its Go service running where BYOS can reach it; see its [README](israel-bus/README.md).

## Kids Age

Shows each child's age as years, months and days, recalculated every time the screen refreshes.
The plugin lives in [`src/`](src/).

```
Adam
Born on: Saturday August 23, 2025
Age today: Saturday February 7, 2026
= 0 years 5 months 15 days
```

### Settings

Up to 4 children. Leave a slot empty to skip it.

| Field | Description | Example |
|-------|-------------|---------|
| `kid_N_name` | Child's name | `Adam` |
| `kid_N_birthday` | Birthday as `YYYY-MM-DD` | `2025-08-23` |

### Layouts

| Layout | Shows |
|--------|-------|
| `full` | Name, born date, today's date, computed age |
| `half_horizontal` | One compact row: name, short date, age summary |
| `half_vertical` | Name, dates, age |
| `quadrant` | Name and age only |

## Repository layout

```
src/          Kids Age plugin (settings.yml + Liquid templates)
israel-bus/   Israel Bus plugin: src/ (plugin), main.go (Go service), board.html
.github/      CI and release workflows
```

## Releases

Each plugin has its own tag scheme, and CI attaches the ZIP:

| Plugin | Tag | Workflow | Attached files |
|---|---|---|---|
| Kids Age | `X.Y.Z`, created by publishing a GitHub release | [`publish-release.yml`](.github/workflows/publish-release.yml) | `kids-age-plugin.zip` |
| Israel Bus | `israel-bus-vX.Y.Z`, pushed as a git tag | [`israel-bus.yml`](.github/workflows/israel-bus.yml) | `israel-bus-plugin.zip`, `israel-bus-linux-arm64` |

## License

See [LICENSE](LICENSE).
