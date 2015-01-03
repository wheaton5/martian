(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.pipestances = [];
    $scope.config = {};
    return $http.get('/api/get-pipestances').success(function(data) {
      $scope.pipestances = data.pipestances;
      return $scope.config = data.config;
    });
  });

}).call(this);
