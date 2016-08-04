
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


/*
 * Handle next/previous page clicks on paginated views.
 */
function changepage(delta) {
	update_model_from_ui()
	
	if (global_view_state.page == null) {
		global_view_state.page  = 0;
	}
	global_view_state.page += delta
	if (global_view_state.page < 0) {
		global_view_state.page = 0;
	}

	global_view_state.render();
}


/*
 * Handle clicks that change the current project.
 */
function project_dropdown_click(x) {
	changeproject(event.target.textContent);
}


function changeproject(p) {
	global_view_state.project = p;
	update_model_from_ui();
	global_view_state.render();
}

/*
 * Handle "basic" and "metrics" clicks
 */
function changetablemode(mode) {
	update_model_from_ui()
	global_view_state.table_mode = mode;
	global_view_state.render();

}

/*
 * Handle clicks on the top-level buttons.
 */
function pickwindow(mode) {
	update_model_from_ui()
	global_view_state.mode = mode;
	global_view_state.render();
}

/*
 * Handles clicks on the "go" button on the chart view 
 */
function update() {
	update_model_from_ui()
	global_view_state.render();

}

/*
 * Handle clicks on the button to change the chart style
 */
function update_chart_style(chart_mode) {
	global_view_state.chart_mode = chart_mode;
	global_view_state.render();

}

/*
 * Handle clicks on the "update" button in playground mode.
 */
function updateprojecttextarea() {
	//update_model_from_ui();
	global_view_state.write_playground();
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
	this.chartx = "SHA";
	this.charty = null;
	this.sample_search = null;
	this.sortby = "finishdate"
	this.chart_mode="line";
	this.page = 0;

	return this;
}

/*
 * Get a URL with a serialized version of this viewstate embedded in it.
 */
ViewState.prototype.GetURL = function() {
	var url = document.location;
	var str = "http://" + url.host + url.pathname + "?params=" +
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
 * This defines the mappings between components of the viewstate object and HTML IDs on the page
 *
 * |model| is the n ame of the field of the ViewState object
 * |element| is the jquery selector for the HTML element that relfects that value
 * |method| is the (jquery selector) method to call on the element to change it.
 */
var model_view_bindings = [
	{model:"sortby", element:"#sortby", method:"val"},
	{model:"chartx", element:"#chartx", method:"val"},
	{model:"charty", element:"#charty", method:"val"},
	{model:"where", element:"#where", method:"val"},
	{model:"project", element:"#project_cur", method:"text"},
	{model:"chart_mode", element:"#chart_mode", method:"text"}
]

/*
 * This function copies fields of the ViewState object back to the DOM, 
 * using model_view_bindings to figure out how to associate ViewState fields
 * with dom elements.
 */
ViewState.prototype.apply_view_bindings= function() {
	for (var i = 0; i < model_view_bindings.length; i++) {
		var b = model_view_bindings[i];
		$(b.element)[b.method](this[b.model])
	}
}


/*
 * This is the master render function. We call this whenever we change the contents of
 * the ViewState object to update the page accordingly.
 */
ViewState.prototype.render = function() {
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();
	$("#playground").hide()
	set_csv_download_url("");

	clear_error_box();

	this.apply_view_bindings();

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
	} else {
		$("#" + w).show();

		if (w == "table") {
			this.table_update();
		}

		if (w == "plot") {
			/* Grab the metric definitions that we want to show as options in the plot view */
			get_json_safe("/api/list_metrics?metrics_def=" + this.project, function(data) {
				global_metrics_db = data.ChartData;
				var mdata = google.visualization.arrayToDataTable(global_metrics_db);
				global_metrics_table.draw(mdata, {})
			})

			this.chart_update();

		}

		if (w == "playground") {
			this.update_playground();
		}

	}
	set_permalink_url(this.GetURL());
}

ViewState.prototype.update_playground = function () {
	var url = "/api/downloadproject?metrics_def=" + this.project;

	get_json_safe(url, function(data) {
		console.log(data);
		var js = JSON.stringify(data.project_def, null, 2);
		var tx = document.getElementById("project_def");
		tx.value = js;
	})
}

ViewState.prototype.write_playground = function () {
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
		set_csv_download_url(url);
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
		var url = "/api/everything?id=" + this.compareidold +
			"&where=" + encodeURIComponent("StageName NOT IN ('REPORT_COVERAGE','REPORT_LENGTH_MASS','_perf')")
	} else if (mode=="details" && this.compareidold) {
		var url = "/api/details?id=" + this.compareidold
	} else {
		var url = "/api/plot?where=" + where + "&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"
	}

	url+="&page=" + this.page;

	url += "&metrics_def=" + this.project;

	get_json_safe(url, function(data) {
		global_table_data = data;
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		//var options = {width: 1200, allowHtml:true};
		var options = {allowHtml:true, cssClassNames: {tableCell:"chart-cell"}};
		colorize_table(data.ChartData, gdata, "OK", "out-of-spec")
		global_table.draw(gdata, options)
		set_csv_download_url(url);

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
		"&sortby=" + encodeURIComponent(sortby) ;

	console.log(url);
	var that=this;
	get_json_safe(url, function(data) {
		console.log("GOTDATA!");
		console.log(data)
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options;
		if (that.chart_mode == 'line') {
			options = {title:data.Name, lineWidth:2, pointSize:2};
		} else {
			options = {title:data.Name, lineWidth:0, pointSize:5};
		}

		global_chart.draw(gdata, options);
		set_csv_download_url(url);
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

function set_permalink_url(l) {
	var link = document.getElementById("myurl");
	link.text = l
	link.href = l
	console.log(l)

}

function set_csv_download_url(l) {
	var csv = document.getElementById("csvlink");

	/* Use empty string to "hide" the link */
	if (l) {
		l = l +"&csv=yes";
	}
	csv.text = l;
	csv.href = l;
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



