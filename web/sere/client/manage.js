(function() {
  var app, callApi, capitalize;

  app = angular.module('app', ['ui.bootstrap']);

  callApi = function($scope, $http, $data, $url) {
    return $http.post($url, $data).success(function(data) {
      $scope.refreshItems();
      if (data) {
        return window.alert(data.toString());
      }
    });
  };

  capitalize = function(str) {
    return str[0].toUpperCase() + str.slice(1);
  };

  app.controller('ManageCtrl', function($scope, $http, $interval, $modal) {
    $scope.admin = admin;
    $scope.data = null;
    $scope.categories = ['lena', 'standard', 'fuzzer', 'custom', 'aggregator', 'reanalyzer'];
    $scope.cols = {
      programs: ['name', 'battery'],
      batteries: ['name', 'tests'],
      tests: ['name', 'category', 'id'],
      packages: ['name', 'target', 'build_date', 'mro_version', 'state']
    };
    $scope.types = _.keys($scope.cols);
    $scope.type = 'programs';
    $scope.refreshItems = function() {
      return $http.get('/api/manage/get-items').success(function(data) {
        return $scope.data = data;
      });
    };
    $scope.refreshItems();
    $scope.getName = function(prop) {
      var i, len, value, values;
      if (typeof prop === 'object') {
        if ('name' in prop) {
          return prop.name;
        }
        values = [];
        for (i = 0, len = prop.length; i < len; i++) {
          value = prop[i];
          values.push($scope.getName(value));
        }
        return values.join(', ');
      }
      return prop;
    };
    $scope.manageItemForm = function(action) {
      var modalInstance;
      modalInstance = $modal.open({
        animation: true,
        templateUrl: 'manage_item.html',
        controller: 'ManageItemCtrl',
        resolve: {
          title: function() {
            return capitalize(action) + ' ' + capitalize($scope.type);
          },
          action: function() {
            return action;
          },
          type: function() {
            return $scope.type;
          },
          categories: function() {
            return $scope.categories;
          },
          data: function() {
            return $scope.data;
          }
        }
      });
      return modalInstance.result.then(function(item) {
        var data, test, url;
        switch ($scope.type) {
          case 'programs':
            data = {
              name: item.name,
              battery: item.battery.name
            };
            url = '/api/manage/' + action + '-program';
            break;
          case 'batteries':
            data = {
              name: item.name,
              tests: (function() {
                var i, len, ref, results;
                ref = item.tests;
                results = [];
                for (i = 0, len = ref.length; i < len; i++) {
                  test = ref[i];
                  results.push(test.name);
                }
                return results;
              })()
            };
            url = '/api/manage/' + action + '-battery';
            break;
          case 'tests':
            data = {
              name: item.name,
              category: item.category,
              id: item.id
            };
            url = '/api/manage/' + action + '-test';
            break;
          case 'packages':
            data = {
              name: item.name,
              target: item.target
            };
            url = '/api/manage/' + action + '-package';
        }
        return callApi($scope, $http, data, url);
      }, null);
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshItems();
      }), 5000);
    }
  });

  app.controller('ManageItemCtrl', function($scope, $modalInstance, title, action, type, categories, data) {
    var item;
    $scope.title = title;
    $scope.action = action;
    $scope.type = type;
    $scope.categories = categories;
    $scope.data = data;
    $scope.names = (function() {
      var i, len, ref, results;
      ref = data[type];
      results = [];
      for (i = 0, len = ref.length; i < len; i++) {
        item = ref[i];
        results.push(item.name);
      }
      return results;
    })();
    $scope.item = {};
    $scope.manageItem = function() {
      return $modalInstance.close($scope.item);
    };
    return $scope.cancelItem = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

}).call(this);
