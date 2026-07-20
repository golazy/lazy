# Tailwind

Edit Tailwind source under `app/styles` and templates under `app/views`. The
compiled stylesheet is generated output.

```css
/* app/styles/application.css */
@import "tailwindcss";

@custom-variant dark (&:where(.dark, .dark *));
```

```gotemplate
{{/* app/views/layouts/app.html.tpl */}}
{{stylesheet "/styles.css"}}
```

```sh
lazy tailwind
```

Run the generator after Tailwind input, package files, or CSS dependencies
change. Commit `app/public/styles.css`; do not edit it by hand. Keep app
styling in utility classes unless a real reusable CSS abstraction is needed.

## Related

[Assets](assets.md) | [Views](views.md) | [Js](js.md)
