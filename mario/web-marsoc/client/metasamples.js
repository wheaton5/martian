(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MarioRunCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.selsample = null;
    $scope.showbutton = true;
    $http.get('/api/get-metasamples').success(function(data) {
      return $scope.samples = data;
    });
    $scope.selectSample = function(sample) {
      var _ref;
      $scope.selsample = sample;
      $scope.selsample.selected = true;
      return $http.post('/api/get-metasample-callsrc', {
        id: (_ref = $scope.selsample) != null ? _ref.id.toString() : void 0
      }).success(function(data) {
        if ($scope.selsample != null) {
          return _.assign($scope.selsample, data);
        }
      });
    };
    return $scope.invokeAnalysis = function() {
      var _ref;
      $scope.showbutton = false;
      return $http.post('/api/invoke-metasample-analysis', {
        id: "" + ((_ref = $scope.selsample) != null ? _ref.id.toString() : void 0)
      }).success(function(data) {
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
  });

}).call(this);
