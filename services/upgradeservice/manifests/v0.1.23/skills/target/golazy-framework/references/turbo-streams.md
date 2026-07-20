# Turbo/Streams

Use `.turbo_stream.tpl` views for one response that updates multiple DOM
targets.

```gotemplate
{{/* app/views/posts/create.turbo_stream.tpl */}}
<turbo-stream action="prepend" target="posts">
  <template>{{ partial "post_card" .post }}</template>
</turbo-stream>

<turbo-stream action="replace" target="post_form">
  <template>{{ turbo_frame "post_form" . }}</template>
</turbo-stream>
```

Keep target IDs stable and render the same partials used by the full HTML page
so the two representations do not drift.

## Related

[Turbo/Streams/Controller](turbo-streams-controller.md) |
[Turbo/Controller](turbo-controller.md) | [Views](views.md)
