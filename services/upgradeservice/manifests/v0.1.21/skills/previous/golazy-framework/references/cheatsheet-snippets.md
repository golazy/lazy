# Cheatsheet Snippets

The public GoLazy cheatsheet is a generated-app maintenance surface. It lives
in the website repository at:

```text
apps/golazy.dev/app/content/guides/latest/cheatsheet.md
```

Use it when a task changes a public API, generated-app convention, or common
sample-app pattern that users are likely to copy.

## Snippet Contract

- Snippets answer "how can I?" for one narrow task.
- Snippets show changes from the generated sample app, not a separate demo app.
- Snippets should be complete enough to copy for that task.
- Code blocks should contain code and comments, not explanatory paragraphs.
- Each snippet should point readers toward the fuller guide through the
  cheatsheet's `See Others` section instead of duplicating guide prose.

## Maintenance Loop

When changing framework behavior, CLI behavior, generated app structure,
controller conventions, forms, SEO, caching, Turbo, assets, mailers, jobs,
services, PWA, MCP, or the sample app skill itself:

1. Update the focused latest guide.
2. Update `cheatsheet.md` when a snippet exists or should be added.
3. Update this skill or its references when agents need the same convention.
4. Add a changelog note when the public guide surface changes.
