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
    $scope.cols = {
      programs: ['name', 'battery'],
      batteries: ['name', 'tests'],
      tests: ['name', 'id'],
      packages: ['name', 'target', 'mro_version']
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
    $scope.createItemForm = function() {
      var modalInstance;
      modalInstance = $modal.open({
        animation: true,
        templateUrl: 'create_item.html',
        controller: 'CreateItemCtrl',
        resolve: {
          title: function() {
            return 'Create ' + capitalize($scope.type);
          },
          cols: function() {
            return $scope.cols;
          },
          type: function() {
            return $scope.type;
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
            url = '/api/manage/create-program';
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
            url = '/api/manage/create-battery';
            break;
          case 'tests':
            data = {
              name: item.name,
              id: item.id
            };
            url = '/api/manage/create-test';
            break;
          case 'packages':
            data = {
              name: item.name,
              target: item.target
            };
            url = '/api/manage/create-package';
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

  app.controller('CreateItemCtrl', function($scope, $modalInstance, title, cols, type, data) {
    $scope.title = title;
    $scope.cols = cols;
    $scope.type = type;
    $scope.data = data;
    $scope.item = {};
    $scope.createItem = function() {
      return $modalInstance.close($scope.item);
    };
    return $scope.cancelItem = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

}).call(this);
