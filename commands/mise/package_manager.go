package mise

import "golazy.dev/lazy/services/nodeservice"

type NodePackageManager = nodeservice.NodePackageManager

var DetectNodePackageManager = nodeservice.DetectNodePackageManager
var DirectNPM = nodeservice.DirectNPM
var QueryRunner = nodeservice.QueryRunner
var CurrentInstalledTools = nodeservice.CurrentInstalledTools
