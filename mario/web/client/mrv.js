(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MarioRunCtrl', function($scope, $http, $interval) {
    $scope.pipestances = [];
    $scope.usermap = {};
    return $http.get('/api/get-pipestances').success(function(data) {
      $scope.pipestances = data.pipestances;
      return $scope.usermap = data.usermap;
    });
  });

}).call(this);
