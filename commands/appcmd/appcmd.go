package appcmd

import "golazy.dev/lazy/services/appservice"

const DefaultViewPath = appservice.DefaultViewPath
const DefaultPublicPath = appservice.DefaultPublicPath

type LazyDevPaths = appservice.LazyDevPaths

var Find = appservice.Find
var ModuleName = appservice.ModuleName
var GoRunArgs = appservice.GoRunArgs
var GoBuildArgs = appservice.GoBuildArgs
var LazyDevBuildFlags = appservice.LazyDevBuildFlags
var LazyDevLDFlags = appservice.LazyDevLDFlags
var ResolveLazyDevPaths = appservice.ResolveLazyDevPaths
var ResolveViewPath = appservice.ResolveViewPath
var ResolvePublicPath = appservice.ResolvePublicPath
