package services

import "golazy.dev/lazy/services/taskservice"

type TaskSet = taskservice.TaskSet
type Inventory = taskservice.Inventory
type Preparer = taskservice.Preparer

var Inspect = taskservice.Inspect
var DiscoverTasks = taskservice.DiscoverTasks
var HasTask = taskservice.HasTask
var TaskName = taskservice.TaskName
var RunTask = taskservice.RunTask
var TaskCommand = taskservice.TaskCommand
