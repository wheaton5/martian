(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.selsample = null;
    $scope.showbutton = true;
    $http.get('/api/get-metasamples').success(function(data) {
      return $scope.samples = data;
    });
    $scope.refreshSamples = function() {
      return $http.get('/api/get-metasamples').success(function(data) {
        return $scope.samples = data;
      });
    };
    $scope.selectSample = function(sample) {
      var s, _i, _len, _ref, _ref1;
      $scope.selsample = sample;
      _ref = $scope.samples;
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        s = _ref[_i];
        s.selected = false;
      }
      $scope.selsample.selected = true;
      return $http.post('/api/get-metasample-callsrc', {
        id: (_ref1 = $scope.selsample) != null ? _ref1.id.toString() : void 0
      }).success(function(data) {
        if ($scope.selsample != null) {
          return _.assign($scope.selsample, data);
        }
      });
    };
    $scope.invokeAnalysis = function() {
      var _ref;
      $scope.showbutton = false;
      return $http.post('/api/invoke-metasample-analysis', {
        id: (_ref = $scope.selsample) != null ? _ref.id.toString() : void 0
      }).success(function(data) {
        $scope.refreshSamples();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.archiveSample = function() {
      var _ref;
      $scope.showbutton = false;
      return $http.post('/api/archive-metasample', {
        id: (_ref = $scope.selsample) != null ? _ref.id.toString() : void 0
      }).success(function(data) {
        $scope.refreshSamples();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.unfailSample = function() {
      var _ref;
      $scope.showbutton = false;
      return $http.post('/api/restart-metasample-analysis', {
        id: (_ref = $scope.selsample) != null ? _ref.id.toString() : void 0
      }).success(function(data) {
        $scope.refreshSamples();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshSamples();
      }), 5000);
    }
  });

}).call(this);
