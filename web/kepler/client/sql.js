(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap', 'ngSanitize', 'ngCsv']);

  app.controller('SqlCtrl', function($scope, $http, $interval) {
    $scope.res = null;
    $scope.query = null;
    $scope.error = null;
    $scope.getResult = function() {
      return $http.post('/api/get-sql', {
        query: $scope.query
      }).success(function(data) {
        if (data['error']) {
          $scope.error = data['error'];
          return $scope.res = null;
        } else {
          $scope.res = data;
          return $scope.error = null;
        }
      }).error(function() {
        return console.log('Server responded with an error for /api/get-sql.');
      });
    };
    $scope.clearResult = function() {
      $scope.res = null;
      return $scope.error = null;
    };
    return $scope.csvResult = function() {
      return [$scope.res.columns].concat($scope.res.rows);
    };
  });

}).call(this);
