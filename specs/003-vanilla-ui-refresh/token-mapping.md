# Token Mapping

This table maps the existing shared stylesheet tokens to a Vanilla-inspired vocabulary while keeping implementation in `frontend/src/app.css`.

| ssh-man token | Proposed role | Vanilla-inspired cue |
|---|---|---|
| `--surface-base` | App background | Base canvas |
| `--surface-panel` | Standard card/panel surface | Application panel |
| `--surface-panel-strong` | Elevated workspace/dialog surface | Raised panel |
| `--surface-highlight` | Tinted selection and hover fill | Highlight strip |
| `--border-subtle` | Default borders | Hairline border |
| `--border-strong` | Focused or selected borders | Active border |
| `--text-default` | Primary readable text | Body text |
| `--text-muted` | Secondary copy | Muted copy |
| `--accent` | Brand/action color | Link/action blue |
| `--accent-strong` | Primary button emphasis | Strong action blue |
| `--status-success` | Connected/healthy state | Positive state |
| `--status-caution` | Reconnecting/warning state | Caution state |
| `--status-danger` | Failed/error state | Negative state |
| `--status-info` | Neutral runtime progress | Information state |
| `--shadow-panel` | Panel shadow | Layered elevation |
| `--radius-panel` | Card/dialog corner radius | Soft rounded container |
| `--radius-control` | Field/button radius | Control rounding |
| `--space-2` | Tight spacing | xx-small spacing |
| `--space-3` | Small spacing | x-small spacing |
| `--space-4` | Standard spacing | small spacing |
| `--space-5` | Comfortable spacing | medium spacing |
| `--space-6` | Large spacing | large spacing |

Implementation notes:

- Existing tokens such as `--bg`, `--bg-panel`, `--muted`, and `--shadow` remain as compatibility aliases during the migration.
- New semantic classes should prefer `is-*`, `state-*`, and `status-*` naming so components can opt into shared visual behavior without scoped styles.
