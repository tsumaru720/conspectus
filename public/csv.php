<?php

require_once('../config.php');
require_once('../app/MySQL.php');

$mysql = new MySQL($config['SQL_HOSTNAME'], $config['SQL_PORT'], $config['SQL_USERNAME'], $config['SQL_PASSWORD'], $config['SQL_DATABASE']);


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
                        <a class="navbar-brand" href="/">Submit - <?php echo $config['SQL_DATABASE']; ?></a> <span style="padding-left: 100px"><a class="navbar-brand" href="csv.php">CSV</a></span>
                        </div>
                </nav>

                <div class="container-fluid">
                        <div class="row">
                                <div class="col">
                                        <div class="container">

        <?php
$display_form = true;


if ($_FILES) {
    $display_form = false;

   if ( isset($_FILES["file"])) {
        if ($_FILES["file"]["error"] > 0) {
            echo "Return Code: " . $_FILES["file"]["error"];
        } else {
            if ($_FILES["file"]["type"] != "text/csv") { echo "Not CSV"; die(); }

            // Initialize list of assets. We'll update this counter when we insert a new record
            $q = $mysql->query("SELECT id,description from asset_list");
            while ($item = $mysql->fetch($q)) {
                $asset[$item['id']]['description'] = $item['description'];
                $asset[$item['id']]['update'] = 0;
            }

            //$_FILES["file"]["tmp_name"]
            $f = fopen($_FILES["file"]["tmp_name"],"r");
            while(! feof($f)) {
                $line = fgetcsv($f);
                $id = $line[0];
                $deposit = $line[2];
                $value = $line[3];

                if (!array_key_exists($id, $asset)) {
                    // Probably not an ID of something, or it doesnt exist
                    continue;
                } else {
                    //Basic check, skip if invalid
                    if (!is_valid($deposit)) { continue; }
                    if (!is_valid($value)) { continue; }

                    if (($deposit == 0) && ($value == 0)) { continue; }

                    $asset[$id]['update'] = 1;
                    $asset[$id]['deposit'] = $deposit;
                    $asset[$id]['value'] = $value;
                    $data = array(':id' => $id, ':deposit' => $deposit, ':latest' => $value);
                    $q = $mysql->query("INSERT INTO `asset_log` (`id`, `asset_id`, `epoch`, `deposit_value`, `asset_value`) VALUES (NULL, :id, CURRENT_TIMESTAMP, :deposit, :latest);", $data);
                }
            }
            fclose($f);

            foreach ($asset as $a) {
                if ($a['update'] == 0) {
                    ?>
                    <div class="alert alert-danger" role="alert">
                        <b><?php echo $a['description']; ?></b> has no update
                    </div>
                    <?php
                }
            }


// Display results
?>
<table class="table table-striped table-sm">
  <thead>
    <tr>
      <th scope="col">ID</th>
      <th scope="col">Asset</th>
      <th scope="col">Deposit</th>
      <th scope="col">Value</th>
    </tr>
  </thead>
  <tbody>
<?php
            foreach ($asset as $k => $a) {
                if ($a['update'] == 1) {
                    ?>
                        <tr>
                          <th scope="row"><?php echo $k; ?></th>
                          <td><?php echo $a['description']; ?></td>
                          <td><?php echo $a['deposit']; ?></td>
                          <td><?php echo $a['value']; ?></td>
                        </tr>
                    <?php
                }
            }

?>
  </tbody>
</table>
<?php

        }
    } else {
         echo "No file selected <br />";
    }
}

if ($display_form == true) {
        ?>

        <form method="post" enctype="multipart/form-data">

            <div class="form-group">
                <label for="csv_input">CSV</label>
                <input type="file" class="form-control-file" id="csv_input" name="file">
            </div>
            <div class="col-8 text-center">
                <button class="btn btn-primary" type="submit">Submit</button>
            </div>
        </form>

    <?php
}



