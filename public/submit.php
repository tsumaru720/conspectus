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

                <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@4.6.0/dist/css/bootstrap.min.css" integrity="sha384-B0vP5xmATw1+K9KRQjQERJvTumQW0nPEzvF6L/Z6nronJ3oUOFUFpCjEUQouq2+l" crossorigin="anonymous">

                <style>
                        body {
                            padding-top: 75px;
                        }

                        main {
                            padding-bottom: 1rem;
                        }

                        .form-control.is-invalid {
                            background-image: none;
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

if ($_POST) {
    $display_form = false;

    $all_pass = 1;
    foreach ($_POST as $k => $v) {
        if ($k == 'date') { continue; } // Date validation is done later
        if (!is_valid($k)) { $all_pass = 0; break; }
    }

    if (!isset($_POST['date'])) {
        $date = date('Y-m-d');
    } else {
        $date = $_POST['date'];
    }

    if (date('Y-m-d', strtotime($date)) != "1970-01-01") {
        $date = date('Y-m-d', strtotime($date)); //Strip extra bits, if there are any
        $SQLDate = $date." 00:00:00";
    }

    if ((!isset($SQLDate)) || (time() < strtotime($date))) {
        $all_pass = 0;
        $invalid_date = true;
    }

    if (!$all_pass) {
        ?>
            <div class="alert alert-danger" role="alert">
                    Please ensure all fields are filled correctly
            </div>
        <?php
        $display_form = true;
        $date = $_POST['date'];
    } else {
        foreach ($_POST as $k => $v) {
            if ($k != 'date') {
                $tmp = explode("_", $k);
                $id = $tmp[0];
                $asset[$id][$tmp[1]] = $v;
            } else {
                $date = $v;
            }
        }

        foreach ($asset as $id => $v) {
            $data = array(':id' => $id, ':deposit' => $v['deposit'], ':latest' => $v['latest']);
            if (isset($SQLDate)) {
                $data[':date'] = $SQLDate;
                $q = $mysql->query("INSERT INTO `asset_log` (`id`, `asset_id`, `epoch`, `deposit_value`, `asset_value`) VALUES (NULL, :id, :date, :deposit, :latest);", $data);
            } else {
                $q = $mysql->query("INSERT INTO `asset_log` (`id`, `asset_id`, `epoch`, `deposit_value`, `asset_value`) VALUES (NULL, :id, CURRENT_TIMESTAMP, :deposit, :latest);", $data);
            }


        }
        ?>
            <div class="alert alert-success" role="alert">
                    All entries have been submitted
            </div>
        <?php

    }
} else {
    $date = date('Y-m-d');
}

if ($display_form == true) {
        ?>
    <script>
        function removeElement(elementId) {
            var element = document.getElementById(elementId + "_form");
            element.parentNode.removeChild(element);
        }
    </script>
    <form method="post">

    <div class="form-group row justify-content-center mb-5">
        <label class="col-sm-2 col-form-label text-right" for="date">Date</label>
        <div class="col-4 text-center font-weight-bold">
            <input type="date" class="form-control form-control-sm <?php echo isset($invalid_date) ? 'is-invalid' : ''; ?>" name="date" id="date"  value="<?php echo $date; ?>" maxlength="10" required>
        </div>
    </div>

    <div class="form-group row">
            <div class="col-4 text-center font-weight-bold">
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
        $q2 = $mysql->query("SELECT * from asset_list WHERE asset_class = :class_id AND closed = '0' ORDER BY description ASC", $data);

        while ($asset = $mysql->fetch($q2)) {
            $data = array(':asset_id' => $asset['id']);
            $q3 = $mysql->query("SELECT deposit_value, asset_value from asset_log WHERE asset_id = :asset_id ORDER BY epoch DESC LIMIT 1", $data);

            $values = $mysql->fetch($q3);
            ?>
            <div class="form-group row" style="margin-bottom: 5px;" id="<?php echo $asset['id']; ?>_form">
                    <div class="col-4 text-right">
                    <span><?php echo '('.$asset['id'].') '.htmlspecialchars($asset['description']); ?></span> -
                    <span class="small"><?php echo htmlspecialchars($asset_class['description']); ?></span>
                    </div>
                    <div class="col-2">
                            <input class="form-control form-control-sm <?php echo !is_valid($asset['id']."_deposit") ? 'is-invalid' : ''; ?>" name="<?php echo $asset['id']."_deposit"; ?>" placeholder="<?php echo $values['deposit_value']; ?>" value="<?php echo isset($_POST[$asset['id']."_deposit"]) ? $_POST[$asset['id']."_deposit"] : ''; ?>"></input>
                    </div>
                    <div class="col-2">
                            <input class="form-control form-control-sm <?php echo !is_valid($asset['id']."_latest") ? 'is-invalid' : ''; ?>" name="<?php echo $asset['id']."_latest"; ?>" placeholder="<?php echo $values['asset_value']; ?>" value="<?php echo isset($_POST[$asset['id']."_latest"]) ? $_POST[$asset['id']."_latest"] : ''; ?>"></input>
                    </div>
                    <a onclick="removeElement(<?php echo $asset['id']; ?>)" href="javascript:void(0);"><small>Delete</small></a>

            </div>
            <?php
        }

    }

    ?>
    <div class="col-8 text-center">
        <button class="btn btn-primary" type="submit">Submit</button>
    </div>

    <?php
    echo "</form>";

}
?>
