# agent-with-skills

A working example that loads skills from a Trae-style directory
(`SKILL.md` files with YAML frontmatter) and wires them into a pi-ai-go
agent run.

## How it works

1. `harness.LoadSkills(dir)` recursively walks `dir`, finds every
   `SKILL.md`, and parses its YAML frontmatter (`name`, `description`,
   `disable-model-invocation`).
2. `harness.FormatSkillsForSystemPrompt(skills)` renders the visible
   skills as an `<available_skills>` block and we inject it into the
   agent's system prompt.
3. The agent has the built-in tools (`read_file` / `write_file` /
   `edit_file` / `bash` / `glob` / `grep` from `agent/tools`). When the
   user's task matches a skill, the model calls `read_file` to load the
   full SKILL.md and follows its instructions.

## Run

```powershell
cd examples\agent-with-skills
go run . -skills "C:\Users\huangyicao\.trae-cn\skills" -query "list open PRs in the current repo" -v
```

Or interactively:

```powershell
go run . -v
query> run gh pr list and summarize the failing CI checks
```

## Flags

| Flag        | Default                                  | Description                                 |
| ----------- | ---------------------------------------- | ------------------------------------------- |
| `-skills`   | `~/.trae-cn/skills` (Windows-aware)      | Directory to load skills from.              |
| `-model`    | `MODEL` / `MODEL_ID` env, else `gpt-4o`  | Model ID. See `resolveModel` in main.go.     |
| `-provider` | `PROVIDER` env                           | Force a provider (overrides auto-detect).   |
| `-base-url` | `BASE_URL` env                           | Override the provider base URL.             |
| `-api-key`  | `API_KEY` env                            | Override the API key directly.              |
| `-query`    | (read from stdin)                        | Single query and exit.                      |
| `-stream`   | `true`                                   | Print text deltas to stdout.                |
| `-v`        | `false`                                  | Log tool/compaction events to stderr.       |

## Configuration via `.env`

The example calls `loadDotEnv(".env")` before flag parsing, so you can
configure everything in a file. Copy `.env.example` to `.env` next to
the example and edit it:

```powershell
copy .env.example .env
notepad .env
```

```ini
# .env
MODEL=claude-sonnet-4-5
ANTHROPIC_API_KEY=sk-ant-...
```

Real env vars in your shell always win over the `.env` file (the
loader only sets a key when it's not already present in the
environment).

## Add a new skill

Drop a `SKILL.md` file into the skills directory:

```markdown
---
name: my-skill
description: "What the skill does, when to use it."
---

# My Skill

When you see X, do Y.
```

The agent will discover it on the next `go run`.

## Supported model IDs (in `resolveModel`)

- `gpt-4o`, `gpt-4o-mini`  (OpenAI)
- `claude-sonnet-4-5`      (Anthropic)
- `deepseek-chat`          (DeepSeek)
- `glm-4.6`                (GLM / z.ai)
- `moonshot-v1-128k`       (Kimi / Moonshot)

Add more entries to the `known` map to support others.
