# Kids Age - TRMNL Plugin

A plugin for [TRMNL](https://usetrmnl.com) e-ink displays that shows your kids' ages, computed daily from their birthdays.

## Display

**Full view** shows for each child:
```
Adam
Born on: Saturday August 23, 2025
Age today: Saturday February 7, 2026
= 0 years 5 months 15 days
```

Smaller layouts (half, quadrant) show progressively compact versions.

## Installation

1. ZIP the contents of the `src/` directory
2. In your [byos_laravel](https://github.com/usetrmnl/byos_laravel) instance, import the ZIP as a new plugin
3. Configure your kids' names and birthdays in the plugin settings

The plugin includes:
- `settings.yml` - Plugin configuration and custom fields schema
- `plugin.php` - PHP logic for computing ages from birthdays using Carbon
- `*.blade.php` - Blade templates for different screen sizes

## Configuration

After importing, the plugin settings let you configure up to 4 children:

| Field | Description | Example |
|-------|-------------|---------|
| `kid_N_name` | Child's name | `Adam` |
| `kid_N_birthday` | Birthday in YYYY-MM-DD format | `2025-08-23` |

Leave fields empty to skip a slot. The plugin automatically computes and displays the age every time the screen refreshes.

## Templates

| Layout | Description |
|--------|-------------|
| `full` | Full details: name, born date, today's date, computed age |
| `half_horizontal` | Compact row: name, short date, age summary |
| `half_vertical` | Medium detail: name, dates, age |
| `quadrant` | Minimal: name and age only |
