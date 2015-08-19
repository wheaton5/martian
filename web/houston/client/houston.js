(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.filter('formatNumber', function() {
    return function(n, d) {
      return Humanize.formatNumber(n, d);
    };
  });

  app.filter('intComma', function() {
    return function(n) {
      return Humanize.intComma(n);
    };
  });

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.pstances = null;
    return $http.get('/api/get-submissions').success(function(data) {
      var d, _i, _len;
      for (_i = 0, _len = data.length; _i < _len; _i++) {
        d = data[_i];
        d.display_date = d.date.substring(0, 10);
        d.display_domain = d.source.split("@")[0];
        d.display_user = d.source.split("@")[1];
      }
      return $scope.subs = data;
    });
  });

}).call(this);
