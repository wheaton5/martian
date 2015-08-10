(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.pstances = null;
    return $http.get('/api/get-pipestances').success(function(data) {
      return $scope.pstances = data;
    });
  });

}).call(this);
