package lazycode

import "golazy.dev/lazy/services/lazycodeservice"

type RewriteFunc = lazycodeservice.RewriteFunc

var RewriteFile = lazycodeservice.RewriteFile
var EnsureImport = lazycodeservice.EnsureImport
var RemoveImport = lazycodeservice.RemoveImport
var HasImport = lazycodeservice.HasImport
var UsesSelector = lazycodeservice.UsesSelector
