(function() {
  var app, initEditor, initSession, renderPipeline;

  app = angular.module('app', ['ui.bootstrap']);

  app.directive('pipeGraph', function() {
    return {
      restrict: 'A',
      scope: {
        pipelineDec: '=pipelineDec'
      },
      link: function(scope, element, attrs) {
        return scope.$watch(attrs.style, function(value) {
          return renderPipeline(scope.pipelineDec);
        });
      }
    };
  });

  renderPipeline = function(pipelineDec) {
    var bindStm, callStm, g, valueExp, _i, _j, _k, _len, _len1, _len2, _ref, _ref1, _ref2;
    g = new dagreD3.Digraph();
    _ref = pipelineDec.callStmList;
    for (_i = 0, _len = _ref.length; _i < _len; _i++) {
      callStm = _ref[_i];
      g.addNode(callStm.id, {
        label: callStm.id
      });
    }
    _ref1 = pipelineDec.callStmList;
    for (_j = 0, _len1 = _ref1.length; _j < _len1; _j++) {
      callStm = _ref1[_j];
      _ref2 = callStm.bindStmList;
      for (_k = 0, _len2 = _ref2.length; _k < _len2; _k++) {
        bindStm = _ref2[_k];
        valueExp = bindStm.valueExp;
        if (valueExp.kind === 'call') {
          g.addEdge(null, valueExp.id, callStm.id, {
            label: bindStm.type
          });
        }
      }
    }
    console.log(d3.select("g#" + pipelineDec.id));
    return (new dagreD3.Renderer()).run(g, d3.select("g#" + pipelineDec.id));
  };

  app.directive('mainAceEditor', function() {
    return {
      restrict: 'E',
      scope: {
        editorInfo: '=editorInfo',
        save: '&save'
      },
      link: function(scope, element, attrs) {
        var e;
        scope.editorInfo.ace = e = initEditor(element[0]);
        e.setOptions({
          maxLines: 1000
        });
        e.on('change', function(e) {
          return scope.$apply(function() {
            return scope.editorInfo.dirty = true;
          });
        });
        e.commands.addCommand({
          name: 'Save',
          bindKey: {
            win: 'Ctrl-S',
            mac: 'Command-S'
          },
          exec: function(e) {
            return scope.save();
          },
          readOnly: false
        });
        return e.commands.addCommand({
          name: 'Undo',
          bindKey: {
            win: 'Ctrl-Z',
            mac: 'Command-Z'
          },
          exec: function(e) {
            return e.undo();
          },
          readOnly: false
        });
      }
    };
  });

  app.directive('includeAceEditor', function() {
    return {
      restrict: 'E',
      scope: {
        editorInfo: '=editorInfo'
      },
      link: function(scope, element, attrs) {
        var e;
        scope.editorInfo.ace = e = initEditor(element[0]);
        e.renderer.setShowGutter(false);
        return e.setReadOnly(true);
      }
    };
  });

  initEditor = function(elem) {
    var editor;
    editor = ace.edit(elem);
    editor.setTheme('ace/theme/clouds');
    editor.setShowFoldWidgets(false);
    editor.setHighlightActiveLine(false);
    return editor;
  };

  initSession = function(einfo, fname, contents) {
    var session;
    session = new ace.EditSession(contents, 'ace/mode/coffee');
    session.setUseWorker(false);
    session.setUndoManager(new ace.UndoManager());
    einfo.ace.setSession(session);
    einfo.dirty = false;
    return einfo.fname = fname;
  };

  app.controller('MarioEdCtrl', function($scope, $http) {
    $scope.mainEditor = {
      fname: 'select file:',
      dirty: false
    };
    $scope.includeEditor = {
      fname: ''
    };
    $scope.availableFnames = [];
    $http.get('/files').success(function(data) {
      return $scope.availableFnames = data;
    });
    $scope.selectFile = function(fname) {
      return $http.post('/load', {
        fname: fname
      }).success(function(data) {
        initSession($scope.mainEditor, fname, data.contents);
        initSession($scope.includeEditor, data.includeFname, data.includeContents);
        $scope.compilerMessages = '';
        return $scope.pipelineDecList = [];
      });
    };
    $scope.selectTab = function(pipelineDec) {
      return renderPipeline(pipelineDec);
    };
    $scope.build = function() {
      if ($scope.mainEditor.fname === 'select file:') {
        window.alert('Select a file first.');
        return;
      }
      return $http.post('/build', {
        fname: $scope.mainEditor.fname
      }).success(function(data) {
        var locMatch, ptree;
        if (typeof data === 'string') {
          locMatch = data.match(/on line (\d+):/);
          if (locMatch != null) {
            $scope.mainEditor.ace.gotoLine(parseInt(locMatch[1]));
          }
          $scope.compilerMessages = data;
          return $scope.pipelineDecList = [];
        } else {
          ptree = data;
          $scope.compilerMessages = "Build successful:\n" + ("    " + ptree.filetypeDecList.length + " filetype declarations\n") + ("    " + ptree.stageDecList.length + " stage declarations\n") + ("    " + ptree.pipelineDecList.length + " pipeline declarations");
          $scope.pipelineDecList = ptree.pipelineDecList;
          if ($scope.pipelineDecList[0] != null) {
            return $scope.pipelineDecList[0].active = true;
          }
        }
      });
    };
    $scope["new"] = function() {
      var fname;
      fname = window.prompt('Enter file name including .mro extension:');
      if (!((fname != null) && fname.length > 0)) {
        return;
      }
      $scope.availableFnames.push(fname);
      initSession($scope.mainEditor, fname, '');
      initSession($scope.includeEditor, '', '');
      $scope.mainEditor.dirty = true;
      $scope.compileMessages = '';
      return $scope.pipelineDecList = [];
    };
    return $scope.save = function() {
      return $http.post('/save', {
        fname: $scope.mainEditor.fname,
        contents: $scope.mainEditor.ace.getValue()
      }).success(function(data) {
        return $scope.mainEditor.dirty = false;
      });
    };
  });

}).call(this);
