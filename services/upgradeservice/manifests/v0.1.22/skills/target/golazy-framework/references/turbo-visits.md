# Turbo/Visits

Import Turbo from app-owned JavaScript and let ordinary links remain valid
without JavaScript.

```js
// app/js/app.js
// golazy:turbo
// golazy:stimulus
```

```gotemplate
{{/* app/views/layouts/app.html.tpl */}}
{{importmap "/assets/importmap.json"}}
<script type="module">import "app.js"</script>
```

```gotemplate
<a href="{{path_for "posts"}}" data-turbo-action="advance">Posts</a>
<a href="{{path_for "home"}}" data-turbo="false">Full reload</a>
```

Server-rendered routes must work correctly before Turbo enhancement is added.

## Related

[Js](js.md) | [lazyshaft](lazyshaft.md) | [Turbo/Forms](turbo-forms.md) |
[Routes](routes.md)
