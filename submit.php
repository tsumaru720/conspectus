<?php

require_once('config.php');
require_once('inc/mysql.php');

$mysql = new MySQL($CONFIG['SQL_HOSTNAME'], $CONFIG['SQL_PORT'], $CONFIG['SQL_USERNAME'], $CONFIG['SQL_PASSWORD'], $CONFIG['SQL_DATABASE']);


function is_valid($field) {
        if ($_POST) {
                if (is_numeric($_POST[$field])) {
                        return 1;
                }
        } else {
		return 1;
	}
        return 0;
}

?>


<!DOCTYPE html>
<html lang="en">
        <head>
                <meta charset="utf-8">
                <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">

                <title>Submit</title>

                <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-alpha.6/css/bootstrap.min.css" integrity="sha384-rwoIResjU2yc3z8GV/NPeZWAv56rSmLldC3R/AZzGRnGxQQKnKkoFVhFQhNUwEyJ" crossorigin="anonymous">

                <style>
                        body {
                                padding-top: 75px;
                        }

                        main {
                                padding-bottom: 1rem;
                        }

                </style>

        </head>

        <body>
                <nav class="navbar navbar-toggleable-md navbar-inverse fixed-top bg-inverse">
                        <a class="navbar-brand" href="submit.php">Submit</a>
                        </div>
                </nav>

                <div class="container-fluid">
                        <div class="row">
                                <div class="col">
                                        <div class="container">

		<?php
$display_form = true;

if ($_POST) {
	$display_form = false;

	$all_pass = 1;
	foreach ($_POST as $k => $v) {
		if (!is_valid($k)) { $all_pass = 0; break; }
	}

	if (!$all_pass) {
		?>
	        <div class="alert alert-danger" role="alert">
	                Please complete all fields
	        </div>
		<?php
		$display_form = true;
	} else {
		foreach ($_POST as $k => $v) {
			$tmp = explode("_", $k);
			$id = $tmp[0];
			$asset[$id][$tmp[1]] = $v;
		}

		foreach ($asset as $id => $v) {
		        $data = array(':id' => $id, ':deposit' => $v['deposit'], ':latest' => $v['latest']);
			$q = $mysql->query("INSERT INTO `asset_log` (`id`, `asset_id`, `epoch`, `deposit_value`, `asset_value`) VALUES (NULL, :id, CURRENT_TIMESTAMP, :deposit, :latest);", $data);

		}
		?>
	        <div class="alert alert-success" role="alert">
	                All entries have been submitted
	        </div>
		<?php

	}

}

if ($display_form == true) {
		?>

	<form method="post">
	<div class="form-group row">
	        <div class="col-3 text-center font-weight-bold">
	                Asset
	        </div>
	        <div class="col-2 text-center font-weight-bold">
	                Deposit Value
	        </div>
	        <div class="col-2 text-center font-weight-bold">
	                Current Value
	        </div>
	</div>

	<?php

	$q = $mysql->query("SELECT * from asset_classes ORDER BY description ASC");
	while ($asset_class = $mysql->fetch($q)) {

	        $data = array(':class_id' => $asset_class['id']);
		$q2 = $mysql->query("SELECT * from asset_list WHERE asset_class = :class_id ORDER BY description ASC", $data);

		while ($asset = $mysql->fetch($q2)) {
		        $data = array(':asset_id' => $asset['id']);
			$q3 = $mysql->query("SELECT deposit_value, asset_value from asset_log WHERE asset_id = :asset_id ORDER BY epoch DESC LIMIT 1", $data);

			$values = $mysql->fetch($q3);
			?>
			<div class="form-group row" style="margin-bottom: 5px;">
			        <div class="col-3 text-right">
			                <span><?php echo $asset['description']; ?></span><br>
					<span class="small"><?php echo $asset_class['description']; ?></span>
			        </div>
			        <div class="col-2 <?php echo !is_valid($asset['id']."_deposit") ? 'has-danger' : ''; ?>">
			                <input class="form-control" name="<?php echo $asset['id']."_deposit"; ?>" placeholder="<?php echo $values['deposit_value']; ?>" value="<?php echo isset($_POST[$asset['id']."_deposit"]) ? $_POST[$asset['id']."_deposit"] : ''; ?>"></input>
			        </div>
			        <div class="col-2 <?php echo !is_valid($asset['id']."_latest") ? 'has-danger' : ''; ?>">
			                <input class="form-control" name="<?php echo $asset['id']."_latest"; ?>" placeholder="<?php echo $values['asset_value']; ?>" value="<?php echo isset($_POST[$asset['id']."_latest"]) ? $_POST[$asset['id']."_latest"] : ''; ?>"></input>
			        </div>
			</div>
			<?php
		}


	}

	?>
	<div class="col-7 text-center">
		<button class="btn btn-primary" type="submit">Submit</button>
	</div>

	<?php
	echo "</form>";

}
?>


