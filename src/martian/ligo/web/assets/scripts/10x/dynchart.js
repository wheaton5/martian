// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;
var global_table;
var global_table_data;
var global_compare;
var global_metrics_db;
var global_metrics_table;

var global_view_state;


function main() {
	global_view_state = new(ViewState);
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart', 'table']});
	google.charts.setOnLoadCallback(function() {

		global_chart= new google.visualization.LineChart(document.getElementById('plot1'));
		global_table = new google.visualization.Table(document.getElementById('table1'));
		global_compare = new google.visualization.Table(document.getElementById('compare1'));
		global_metrics_table= new google.visualization.Table(document.getElementById('list1'));
		google.visualization.events.addListener(global_metrics_table, 'select', global_view_state.metrics_list_click);

		//pickwindow("table")
		var p = getParameterByName("params");
		if (p != null && p != "") {
			global_view_state.ReconstituteFromURL(p);

		}
		global_view_state.render();
	});
	

	setup_project_dropdown();

}


/*
 * This is a JSON front-end to help with error handling. We call a path and expect
 * an object that looks like:
 * {ERROR:...,
 *  STUFF:...
 * }
 * If error is missing or null, we call success with the STUFF as its only argument.
 * If error is not missing, we render the error in the friendly error box.
 */
function get_json_safe(path, success) {

	/*
	var url = path + "?";
	var first=true;
	for (var key in params) {
		if (!first) {
			url +="&";
		}
		first=false;
		url = url + key + "=" + encodeURIComponent(params[key])
	}
	*/

	var url = path;

	$.getJSON(url, function(data) {
		if (data["ERROR"]) {
			set_error_box(data["ERROR"])
		} else {
			success(data["STUFF"]);
		}
	})
}

/*
 * Handle clicks that change the current project.
 */
function project_dropdown_click(x) {
	console.log(this);
	console.log(event)

	//document.getElementById("metricset").value = event.target.textContent
	changeproject(event.target.textContent);

}

/*
 * Grab the data to fill in the project dropdown.
 */
function setup_project_dropdown() {
	get_json_safe("/api/list_metric_sets", function(data) {
		var pd = $("#projects_dropdown")

		for (var i = 0; i < data.length; i++) {

			var ng = document.createElement('li');
			ng.textContent = data[i];
			ng.onclick = project_dropdown_click;
			console.log(ng.textContent);
			pd.append(ng);
		}
	})
}

function changeproject(p) {
	global_view_state.project = p;
	update_model_from_ui();
	global_view_state.render();
}

function changetablemode(mode) {
	update_model_from_ui()
	global_view_state.table_mode = mode;
	global_view_state.render();

}
function pickwindow(mode) {
	update_model_from_ui()
	global_view_state.mode = mode;
	global_view_state.render();
}

function update() {
	update_model_from_ui()
	global_view_state.render();

}

function updateprojecttextarea() {
	//update_model_from_ui();
	global_view_state.write_override();
	//global_view_state.render();
}

	


/*
 * The view state object tracks *ALL* of the data that defines the current view.
 * It can be serialized (as JSON) and then the page can be reconstituded from it.
 */
function ViewState() {
	this.mode = "table";
	this.table_mode = "";
	this.where = "";
	this.project = "Default.json";
	this.compareidnew= null;
	this.compareidold= null;
	this.chartx = null;
	this.charty = null;
	this.sample_search = null;
	this.sortby = null;

	return this;
}

/*
 * Get a URL with a serialized version of this viewstate embedded in it.
 */
ViewState.prototype.GetURL = function() {
	var url = document.location;
	var str = url.host + url.pathname + "?params=" +
		encodeURIComponent(JSON.stringify(this));
	return str;
}

/*
 * Unpack a serialized version of a ViewState object.
 */
ViewState.prototype.ReconstituteFromURL = function(p) {
	var p = decodeURIComponent(p);

	var parsed = JSON.parse(p);
	
	var ks = Object.keys(parsed);

	for (var i = 0; i < ks.length; i++) {
		this[ks[i]] = parsed[ks[i]]
	}
}


/*
 * This is the master render function.
 */
ViewState.prototype.render = function() {
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();
	$("#override").hide()

	clear_error_box();

	var w = this.mode;

	/* Special logic for handling the compare button. If you don't have exactly two
	 * rows selected, don't compare. If you have run row selected, redo the table view
	 * with selecting rows with the same sampleid
	 */
	if (w == "compare") {
		if (!this.compareidnew || !this.compareidold) {
			set_error_box("Please select two rows to compare. Then click compare again.")
			//var wc=(get_data_at_row(global_table_data, "sampleid", selected[0].row));
			if (this.compareidold) {
				this.where = "sampleid='" + this.sample_search+"'"
				console.log(this.where);
			}

			this.table_update();
			$("#table").show();
		} else{
			this.compare_update();
			$("#compare").show();
		}
	}
	else {

		$("#" + w).show();

		if (w == "table") {
			this.table_update();
		}

		if (w == "plot") {
			get_json_safe("/api/list_metrics?metrics_def=" + this.project, function(data) {
				global_metrics_db = data.ChartData;
				var mdata = google.visualization.arrayToDataTable(global_metrics_db);
				global_metrics_table.draw(mdata, {})
			})

			this.chart_update();

		}

		if (w == "override") {
			this.update_override();
		}

	}
	$("#project_cur").text(this.project);
	$("#myurl").text(this.GetURL());
}

ViewState.prototype.update_override = function () {
	var url = "/api/downloadproject?metrics_def=" + this.project;

	get_json_safe(url, function(data) {
		console.log(data);
		var js = JSON.stringify(data.project_def, null, 2);
		var tx = document.getElementById("project_def");
		tx.value = js;
	})
}

ViewState.prototype.write_override = function () {
	var url = "/api/tmpproject";
	var data = document.getElementById("project_def").value;

	$.post(url, "project_def=" + encodeURIComponent(data), function(res) {
		console.log(res);
		var t=JSON.parse(res);
		console.log(t);
		clear_error_box();
		if (t["ERROR"]) {
			set_error_box(t.ERROR);
			return;
		}

		if (t["STUFF"]) {
			changeproject(t.STUFF.project_id);
			console.log(this.project);
			return;
		}
		set_error_box("unineligable output from server");

	})
}


/*
 * Render the compare view */
ViewState.prototype.compare_update = function() {

	//var selected = global_table.getSelection();
	//console.log(selected)
	//var idold = get_data_at_row(global_table_data, "test_reports.id", selected[0].row);
	//var idnew= get_data_at_row(global_table_data, "test_reports.id", selected[1].row);

	
	/*var url = "/api/compare?base=" + document.getElementById("idold").value +
		"&new=" + document.getElementById("idnew").value +
		"&metrics_def=met1.json"
		*/
	var url = "/api/compare?base=" + this.compareidold +
		"&new=" + this.compareidnew+ 
		"&metrics_def=" + this.project;
	
	console.log(url)
	get_json_safe(url, function(data) {
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {allowHtml:true};
		colorize_table(data.ChartData,gdata, "Diff", "different")
		colorize_table2(data.ChartData,gdata, "OldOK" ,"BaseVal", "out-of-spec");
		colorize_table2(data.ChartData,gdata, "NewOK" ,"NewVal", "out-of-spec");
		global_compare.draw(gdata, options)


	})
}

/* Render the various table views.*/
ViewState.prototype.table_update = function()  { 
	var where = this.where;

	var mode = this.table_mode;
	var id;

	/* Which table view are we actually rendering?*/
	if (mode=="metrics") {
		var url = "/api/plotall?where=" + where 
	} else if (mode=="everything" && this.compareidold) {
		/* XXX Need to show an error if this.compareidold is null */
		var url = "/api/details?id=" + this.compareidold +
			"&where=" + encodeURIComponent("StageName NOT IN ('REPORT_COVERAGE','REPORT_LENGTH_MASS','_perf')")
	} else {
		var url = "/api/plot?where=" + where + "&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"
	}

	url += "&metrics_def=" + this.project;

	get_json_safe(url, function(data) {
		global_table_data = data;
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		//var options = {width: 1200, allowHtml:true};
		var options = {allowHtml:true, cssClassNames: {tableCell:"chart-cell"}};
		colorize_table(data.ChartData, gdata, "OK", "out-of-spec")
		global_table.draw(gdata, options)

	})
}

/*
 * Handle a click on the table of metrics in the chart page
 */
ViewState.prototype.metrics_list_click = function() {
	var y = document.getElementById("charty");
	
	var sel = global_metrics_table.getSelection();
	var v = "";
	for (var i = 0; i < sel.length; i++) {
		if (v != "") {
			v = v + ",";
		}
		var idx = global_metrics_table.getSelection()[i].row;

		v = v  +global_metrics_db[idx+1];
	}
	y.value = v;
}

/*
 * THIS IS PROBABLY THE DROID YOU ARE LOOKING FOR!
 * This function binds the UI to a global instance of the ViewState object. 
 * We call it whenever an interesting change happens and then subseqnetly call the
 * render function to actually update the UI.
 */
function update_model_from_ui() {
	var v = global_view_state;
	v.chartx = document.getElementById("chartx").value;
	v.charty = document.getElementById("charty").value;
	v.where = document.getElementById("where").value;
	v.sortby= document.getElementById("sortby").value;
	var selected = global_table.getSelection();
	if (selected[0]) {
		v.sample_search=(get_data_at_row(global_table_data, "sampleid", selected[0].row));

		v.compareidold= get_data_at_row(global_table_data, "test_reports.id", selected[0].row);
	} else {
		v.compareidold = null;
	}
	if (selected[1]) {
		v.compareidnew= get_data_at_row(global_table_data, "test_reports.id", selected[1].row);
	} else {
		v.compareidnew = null;
	}

}

/*
 * Update the chart.
 */
ViewState.prototype.chart_update = function() {
	var x = this.chartx;
	var y = this.charty;
	var where = this.where
	var sortby = this.sortby || ""

	var url = "/api/plot?where="+encodeURIComponent(where)+
		"&columns=" + encodeURIComponent(x) + "," + encodeURIComponent(y) +
		"&metrics_def=" + this.project +
		"&sortby=" + sortby ;

	console.log(url);
	get_json_safe(url, function(data) {
		console.log("GOTDATA!");
		console.log(data)
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {title:data.Name};
		global_chart.draw(gdata, options);


	})
}

/*
 * Extract data from a specific row and a named columns from a chartdata-like
 * object.
 */
function get_data_at_row(data, columnname, rownumber) {
	var labels = data.ChartData[0];

	var index;
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] == columnname) {
			index = i;
			break;
		}
	}

	return data.ChartData[rownumber+1][index];
}

function find_column_index(data, name) {

	var labels = data[0];

	/* Figure out which column is called "diff" */
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] == name) {
			return i
		}
	}
	console.log("Sorry. cant find: " + name);

}

/*
 * Set colorization for the comparison page.
 */
function colorize_table(data, datatable, column_name, style) {
	var diff_index;
	var labels = data[0];

	/* Figure out which column is called "diff" */
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] == column_name) {
			diff_index= i;
			break;
		}
	}

	/* Look at every row, if its diff column is falst, then color
	 * everything in that row red.
	 */
	for (var i = 1; i < data.length; i++) {
		var di = i - 1;
		
		if (data[i][diff_index] === false) {
			for (var j = 0; j < labels.length; j++) {
			var old = datatable.getProperty(i - 1, j, 'className');
			var ns = "";
			if (old != null) {
				ns = old + " ";
			}
			ns += style;
			datatable.setProperty(i - 1, j, 'className', ns);
	//				datatable.setProperty(di, j, 'className', style)
			}
		}
	}
}

function colorize_table2(data, datatable, column_from, column_to, style) {
	var from = find_column_index(data, column_from);
	var to = find_column_index(data, column_to);


	for (var i = 0; i < data.length; i++) {
		if (data[i][from] === false) {
			var old = datatable.getProperty(i - 1, to, 'className');
			var ns = "";
			if (old != null) {
				ns = old + " ";
			}
			ns += style;

			datatable.setProperty(i - 1, to, 'className', ns);
		}
	}
}

function reload() {
	document.location="/api/reload_metrics"

}

function set_error_box(s) {
	$("#errortext").text(s);
	$("#errorbox").show();
}

function clear_error_box() {
	$("#errorbox").hide();
}


/* Shamelessly stolen from stackoverflow */
function getParameterByName(name, url) {
    if (!url) url = window.location.href;
    name = name.replace(/[\[\]]/g, "\\$&");
    var regex = new RegExp("[?&]" + name + "(=([^&#]*)|&|#|$)"),
        results = regex.exec(url);
    if (!results) return null;
    if (!results[2]) return '';
    return decodeURIComponent(results[2].replace(/\+/g, " "));
}
main();



