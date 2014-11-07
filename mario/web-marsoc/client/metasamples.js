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
      console.log($scope.selsample.id);
      return $http.post('/api/get-metasample-callsrc', {
        id: "" + ((_ref = $scope.selsample) != null ? _ref.id : void 0)
      }).success(function(data) {
        var _ref;
        console.log(data);
        return (_ref = $scope.selsample) != null ? _ref.callsrc = data : void 0;
      });
    };
    return $scope.invokeAnalysis = function() {
      $scope.showbutton = false;
      return $http.post('/api/invoke-analysis', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
  });

}).call(this);
