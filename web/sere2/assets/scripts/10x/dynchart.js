// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;
var global_table;
var global_table_data;
var global_compare;
var global_metrics_db;
var global_metrics_table;



function main() {
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart', 'table']});
	google.charts.setOnLoadCallback(function() {

		global_chart= new google.visualization.LineChart(document.getElementById('plot1'));
		global_table = new google.visualization.Table(document.getElementById('table1'));
		global_compare = new google.visualization.Table(document.getElementById('compare1'));
		global_metrics_table= new google.visualization.Table(document.getElementById('list1'));
		google.visualization.events.addListener(global_metrics_table, 'select', metrics_list_click);

		pickwindow("table")
	});
	
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();
	clear_error_box();


}

function getmet() {
	return document.getElementById("metricset").value;
}

/*
 * This is called on a click to any of the main navication buttons. |w| is the name
 * of the nav button that was clicked.
 */
function pickwindow(w) {
	console.log(w)
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();
	clear_error_box();

	/* Special logic for handling the compare button. If you don't have exactly two
	 * rows selected, don't compare. If you have run row selected, redo the table view
	 * with selecting rows with the same sampleid
	 */
	if (w == "compare") {
		var selected = global_table.getSelection();
		if (selected.length!=2) {
			set_error_box("Please select two rows to compare. Then click compare again.")
			var wc=(get_data_at_row(global_table_data, "sampleid", selected[0].row));
			document.getElementById("where").value="sampleid =" + wc;

			table_update();
			$("#table").show();
		} else if (selected.length == 2){
			compare_update();
			$("#compare").show();
		}
	}
	else {

		$("#" + w).show();

		if (w == "table") {
			table_update();
		}

		if (w == "plot") {
			$.getJSON("/api/list_metrics?metrics_def=" + getmet(), function(data) {
				global_metrics_db = data.ChartData;
				var mdata = google.visualization.arrayToDataTable(global_metrics_db);
				global_metrics_table.draw(mdata, {})
			})

		}
	}
}

/*
 * Render the compare view */
function compare_update() {

	var selected = global_table.getSelection();
	console.log(selected)
	var idold = get_data_at_row(global_table_data, "test_reports.id", selected[0].row);
	var idnew= get_data_at_row(global_table_data, "test_reports.id", selected[1].row);

	
	/*var url = "/api/compare?base=" + document.getElementById("idold").value +
		"&new=" + document.getElementById("idnew").value +
		"&metrics_def=met1.json"
		*/
	var url = "/api/compare?base=" + idold + 
		"&new=" + idnew + 
		"&metrics_def=" + getmet();
	
	console.log(url)
	$.getJSON(url, function(data) {
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {allowHtml:true};
		colorize_table(data.ChartData,gdata)
		global_compare.draw(gdata, options)


	})


}

/* Render the metric view */
function table_update(mode) {
	var where = encodeURIComponent(document.getElementById("where").value);


	if (mode=="metrics") {
		var url = "/api/plotall?where=" + where 
	} else {
		var url = "/api/plot?where=" + where + "&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"
	}

	url += "&metrics_def=" + getmet();

	$.getJSON(url, function(data) {
		global_table_data = data;
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {width: 1200};
		global_table.draw(gdata, options)
	})
}

/*
 * Handle a click on the table of metrics in the chart page
 */
function metrics_list_click() {
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
 * Update the chart.
 */
function chart_update() {
	var x = document.getElementById("chartx");
	var y = document.getElementById("charty");
	var where = document.getElementById("where");

	console.log(x.value);
	console.log(x);
	console.log(y);
	console.log(where);

	var url = "/api/plot?where="+encodeURIComponent(where.value)+
		"&columns=" + encodeURIComponent(x.value) + "," + encodeURIComponent(y.value) +
		"&metrics_def=" + getmet();

	console.log(url);
	$.getJSON(url, function(data) {
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

/*
 * Set colorization for the comparison page.
 */
function colorize_table(data, datatable) {
	var diff_index;
	var labels = data[0];

	/* Figure out which column is called "diff" */
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] == 'Diff') {
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
				datatable.setProperty(di, j, 'style', 'color:red;')
			}
		}
	}
}

function set_error_box(s) {
	$("#errortext").text(s);
	$("#errorbox").show();
}

function clear_error_box() {
	$("#errorbox").hide();
}

main();
