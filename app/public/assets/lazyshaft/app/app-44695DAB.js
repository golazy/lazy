import "@hotwired/turbo"
import { Application } from "@hotwired/stimulus"
import DebouncedFormController from "controllers/debounced_form_controller.js"
import DepgraphController from "controllers/depgraph_controller.js"
import PanelCloseController from "controllers/panel_close_controller.js"
import PanelNavController from "controllers/panel_nav_controller.js"
import PanelResizeController from "controllers/panel_resize_controller.js"
import RouteVisitController from "controllers/route_visit_controller.js"
import TableResizeController from "controllers/table_resize_controller.js"

const application = Application.start()
application.register("debounced-form", DebouncedFormController)
application.register("depgraph", DepgraphController)
application.register("panel-close", PanelCloseController)
application.register("panel-nav", PanelNavController)
application.register("panel-resize", PanelResizeController)
application.register("route-visit", RouteVisitController)
application.register("table-resize", TableResizeController)
