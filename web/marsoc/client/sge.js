(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('QWebCtrl', function($scope, $http, $interval) {
    $scope.qstat = null;
    return $http.get('/api/qstat').success(function(data) {
      console.log(data);
      if (typeof data === "string") {
        return window.alert(data);
      } else {
        return $scope.qstat = data;
      }
    });
  });

}).call(this);
