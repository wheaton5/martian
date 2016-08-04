(function() {
  var app, callApi, callApiWithConfirmation;

  app = angular.module('app', ['ui.bootstrap']);

  callApiWithConfirmation = function($scope, $http, $p, $url) {
    var psid;
    $scope.showbutton = false;
    psid = window.prompt("Please type the sample ID to confirm");
    if (psid === $p.psid) {
      return callApi($scope, $http, $p, $url);
    } else {
      window.alert("Incorrect sample ID");
      return $scope.showbutton = true;
    }
  };

  callApi = function($scope, $http, $p, $url) {
    $scope.showbutton = false;
    return $http.post($url, {
      product: $p.product,
      fcid: $p.fcid,
      pipeline: $p.pipeline,
      psid: $p.psid
    }).success(function(data) {
      $scope.refreshPipestances();
      if (data) {
        return window.alert(data.toString());
      }
    });
  };

  app.controller('PipestancesCtrl', function($scope, $http, $interval) {
    var i, len, prop, ref;
    $scope.admin = admin;
    $scope.state = state;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.autoinvoke = {
      button: true,
      state: false
    };
    $scope.props = ['name', 'fcid', 'pipeline', 'psid', 'state'];
    $scope.showbutton = true;
    $scope.name = null;
    $scope.fcid = null;
    $scope.pipeline = null;
    $scope.psid = null;
    $scope.fpipestances = [];
    $scope.pipestances = [];
    $scope.pmax = 50;
    $scope.pavailable = 3;
    $scope.pindex = 0;
    $scope.ptotal = 0;
    $scope.previousPage = function() {
      if ($scope.pindex > 0) {
        return $scope.pindex--;
      }
    };
    $scope.setPage = function(pindex) {
      if (0 <= pindex && pindex <= $scope.ptotal.length - 1) {
        return $scope.pindex = pindex;
      }
    };
    $scope.nextPage = function() {
      if ($scope.pindex < $scope.ptotal.length - 1) {
        return $scope.pindex++;
      }
    };
    $scope.refreshPipestances = function() {
      $http.get('/api/get-pipestances').success(function(data) {
        var fcids, i, len, names, p, pipelines, psids, ref;
        $scope.pipestances = _.sortBy(data, function(p) {
          return [p.fcid, p.pipeline, p.psid, p.state];
        });
        $scope.filterPipestances();
        names = {};
        fcids = {};
        pipelines = {};
        psids = {};
        ref = $scope.pipestances;
        for (i = 0, len = ref.length; i < len; i++) {
          p = ref[i];
          names[p.name] = true;
          fcids[p.fcid] = true;
          pipelines[p.pipeline] = true;
          psids[p.psid] = true;
        }
        $scope.names = _.keys(names);
        $scope.fcids = _.keys(fcids);
        $scope.pipelines = _.keys(pipelines);
        $scope.psids = _.keys(psids);
        return $scope.showbutton = true;
      });
      return $http.get('/api/get-auto-invoke-status').success(function(data) {
        $scope.autoinvoke.state = data.state;
        return $scope.autoinvoke.button = true;
      });
    };
    $scope.filterPipestances = function() {
      var filter, i, j, len, len1, p, prop, ref, ref1;
      $scope.fpipestances = [];
      ref = $scope.pipestances;
      for (i = 0, len = ref.length; i < len; i++) {
        p = ref[i];
        filter = true;
        ref1 = $scope.props;
        for (j = 0, len1 = ref1.length; j < len1; j++) {
          prop = ref1[j];
          if ($scope[prop] && $scope[prop] !== p[prop]) {
            filter = false;
          }
        }
        if (filter) {
          $scope.fpipestances.push(p);
        }
      }
      return $scope.ptotal = _.range(Math.floor(($scope.fpipestances.length + $scope.pmax - 1) / $scope.pmax));
    };
    $scope.refreshPipestances();
    ref = $scope.props;
    for (i = 0, len = ref.length; i < len; i++) {
      prop = ref[i];
      $scope.$watch(prop, function() {
        $scope.pindex = 0;
        return $scope.filterPipestances();
      });
    }
    $scope.filterPage = function(pindex) {
      if (pindex < $scope.pindex * $scope.pmax || ($scope.pindex + 1) * $scope.pmax <= pindex) {
        return false;
      }
      return true;
    };
    $scope.invokePipestance = function(p) {
      return callApi($scope, $http, p, '/api/invoke-sample');
    };
    $scope.archivePipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/archive-sample');
    };
    $scope.wipePipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/wipe-sample');
    };
    $scope.killPipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/kill-sample');
    };
    $scope.unfailPipestance = function(p) {
      return callApi($scope, $http, p, '/api/restart-sample');
    };
    $scope.capitalize = function(str) {
      return str[0].toUpperCase() + str.slice(1);
    };
    $scope.getAutoInvokeClass = function() {};
    if ($scope.autoinvoke.state) {
      return "complete";
    } else {
      return "failed";
    }
    $scope.setAutoInvoke = function() {
      $scope.autoinvoke.button = false;
      return $http.post('/api/set-auto-invoke-status', {
        state: !$scope.autoinvoke.state
      }).success(function(data) {
        $scope.refreshPipestances();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshPipestances();
      }), 5000);
    }
  });

}).call(this);
