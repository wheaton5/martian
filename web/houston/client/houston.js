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

  app.controller('MartianRunCtrl', function($scope, $http, $interval, $element) {
    $scope.pstances = null;
    $scope.pipestanceFilter = false;
    $http.get('/api/get-submissions').success(function(data) {
      var d, i, len;
      for (i = 0, len = data.length; i < len; i++) {
        d = data[i];
        d.display_date = d.date.substring(0, 10);
        d.display_domain = d.source.split("@")[0];
        d.display_user = d.source.split("@")[1];
      }
      return $scope.subs = data;
    });
    return $scope.togglePipestanceFilter = (function(event) {
      var el, i, j, len, len1, ref, ref1;
      event.preventDefault();
      if ($scope.pipestanceFilter) {
        ref = document.getElementsByClassName('fileRow');
        for (i = 0, len = ref.length; i < len; i++) {
          el = ref[i];
          angular.element(el).removeClass('hidden');
        }
        return $scope.pipestanceFilter = false;
      } else {
        ref1 = document.getElementsByClassName('fileRow');
        for (j = 0, len1 = ref1.length; j < len1; j++) {
          el = ref1[j];
          angular.element(el).addClass('hidden');
        }
        return $scope.pipestanceFilter = true;
      }
    });
  });

}).call(this);
