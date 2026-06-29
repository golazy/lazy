import "@hotwired/turbo"
import { Application } from "@hotwired/stimulus"
import DebouncedFormController from "controllers/debounced_form_controller.js"
import PanelCloseController from "controllers/panel_close_controller.js"
import PanelResizeController from "controllers/panel_resize_controller.js"
import TableResizeController from "controllers/table_resize_controller.js"

const application = Application.start()
application.register("debounced-form", DebouncedFormController)
application.register("panel-close", PanelCloseController)
application.register("panel-resize", PanelResizeController)
application.register("table-resize", TableResizeController)
