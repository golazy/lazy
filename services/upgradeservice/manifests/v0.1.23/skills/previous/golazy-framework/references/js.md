# Js

Keep app-owned browser modules under `app/js`. Prefer server-rendered HTML with
small Stimulus controllers attached through data attributes.

```js
// app/js/app.js
// golazy:turbo
// golazy:stimulus
```

```js
// app/js/controllers/counter_controller.js
import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["output"]
  static values = { count: { type: Number, default: 0 } }

  increment() {
    this.countValue += 1
    this.outputTarget.textContent = String(this.countValue)
  }
}
```

```gotemplate
<div data-controller="counter">
  <output data-counter-target="output">0</output>
  <button type="button" data-action="counter#increment">Increment</button>
</div>
```

Run `lazy js` after changing `app/js`, `js.toml`, package files, or browser
dependencies. Business behavior remains server-side in services; JavaScript
owns interaction and presentation state.

## Related

[lazyshaft](lazyshaft.md) | [Turbo/Visits](turbo-visits.md) |
[Assets](assets.md) | [Views](views.md)
