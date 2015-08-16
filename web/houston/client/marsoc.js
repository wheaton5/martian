(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.pstances = null;
    return $http.get('/api/get-pipestances').success(function(data) {
      var d, _i, _len;
      for (_i = 0, _len = data.length; _i < _len; _i++) {
        d = data[_i];
        d.display_date = d.date.substring(0, 10);
      }
      return $scope.pstances = data;
    });
  });

}).call(this);
