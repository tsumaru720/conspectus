<?php

class Document extends Theme {

    protected $pageTitle = 'Payments';
    private $db = null;
    private $twig = null;

    public function __construct(&$main, &$twig, $vars) {

        $vars['page_title'] = $this->pageTitle;

        $this->twig = $twig;
        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();

        $q = $this->db->query("SELECT * from asset_list ORDER BY description ASC");
        while ($item = $this->db->fetch($q)) {
            $vars['asset_menu_items'][] = $item;
        }

        if ($vars['action'] == "new") {
            $this->pageTitle .= " - New Payment";
        } elseif ($vars['action'] == "edit") {
            $this->pageTitle .= " - Edit Payment";
        } elseif ($vars['action'] == "delete") {
            $this->pageTitle .= " - Delete Payment";
        }

        $this->vars = $vars;

        // Other requests types should never get this far
        // due to bramus router matching.
        if ($_SERVER['REQUEST_METHOD'] == "GET") {
            $this->processGet($vars['action']);
        } elseif ($_SERVER['REQUEST_METHOD'] == "POST") {
            $this->processPost($vars['action']);
        }

    }

    private function processGet($action) {
        if ($this->vars['left_menu'] == 'all') {
            $this->vars['item_id'] = 0;
        }

        if ($action == "new") {
            if ($this->vars['item_id'] > 0) {
                $asset = $this->entityManager->getAsset($this->vars['item_id']);
                $this->vars['data_asset'] = $asset->getID();
            }
            $this->vars['data_date'] = date('Y-m-d');

            $this->document = $this->twig->load('payment_manager.html');
        } elseif ($action == "edit") {
            $payment = $this->entityManager->getPayment($this->vars['payment_id']);
            $this->vars['data_asset'] = $payment->getAssetID();
            $this->vars['data_date'] =  date('Y-m-d', strtotime($payment->getEpoch()));
            $this->vars['data_amount'] = $payment->getAmount();

            $this->document = $this->twig->load('payment_manager.html');
        } elseif ($action == "delete") {
            $payment = $this->entityManager->getPayment($this->vars['payment_id']);
            $this->vars['data_asset'] = $payment->getDescription();
            $this->vars['data_date'] =  date('jS F Y', strtotime($payment->getEpoch()));
            $this->vars['data_amount'] = $payment->getAmount();

            $this->document = $this->twig->load('payment_delete.html');
        } else {
            // Should probably handle this nicer
            // Chances are someone's trying to break something
            // So meh.
            die();
        }
    }

    private function processPost($action) {
        if ($action == "new") {
            if ($data = $this->assetValidation()) {
                $this->db->query("INSERT INTO `payments` (`id`, `asset_id`, `epoch`, `amount`) VALUES (NULL, :asset, :date, :amount);", $data);
            }
            $this->vars['data_date'] = date('Y-m-d');
            $this->vars['success_msg'] = "Added";

            $this->document = $this->twig->load('payment_manager.html');
        } elseif ($action == "edit") {
            if ($data = $this->assetValidation()) {
                if (!$payment = $this->entityManager->getPayment($this->vars['payment_id'])) {
                    // Should probably handle this nicer
                    // Chances are someone's trying to break something
                    // So meh.
                    die();
                }
                $data[':payment_id'] = $payment->getID();
                $this->db->query("UPDATE `payments` SET `asset_id` =  :asset, `epoch` = :date, `amount` = :amount WHERE `payments`.`id` = :payment_id;", $data);
            }
            $this->vars['data_asset'] = $data[':asset'];
            $this->vars['data_date'] =  date('Y-m-d', strtotime($data[':date']));
            $this->vars['data_amount'] = $data[':amount'];
            $this->vars['success_msg'] = "Edited";

            $this->document = $this->twig->load('payment_manager.html');
        } elseif ($action == "delete") {
            if (!$payment = $this->entityManager->getPayment($this->vars['payment_id'])) {
                // Should probably handle this nicer
                // Chances are someone's trying to break something
                // So meh.
                die();
            }
            $data = array(':payment_id' => $payment->getID());
            $this->db->query("DELETE FROM `payments` WHERE `payments`.`id` = :payment_id;", $data);
            header('Location: /ledger/asset/'.$payment->getAssetID());
        } else {
            // Should probably handle this nicer
            // Chances are someone's trying to break something
            // So meh.
            die();
        }
    }

    private function validateInput($key) {
        // Simple validation we can apply to everything
        if (!array_key_exists($key, $_POST)) {
            return false;
        } else {
            $_POST[$key] = trim($_POST[$key]);
            if ($_POST[$key] == "") {
                return false;
            }
        }
        return $_POST[$key];
    }

    private function assetValidation() {
            $date = $this->validateInput('date');
            $asset = $this->validateInput('asset');
            $amount = $this->validateInput('amount');
            $validated = true;

            if (($date == false) || ($asset == false) || ($amount == false)) {
                $validated = false;
                $this->vars['error_code'] = "EMPTY";
                $this->vars['error_string'] = "Input is empty";
            }

            // Date input validation
            if ($validated == true) { //Only do this if previous check passes
                if (date('Y-m-d', strtotime($date)) != "1970-01-01") {
                    $date = date('Y-m-d', strtotime($date)); //Strip extra bits, if there are any
                    $SQLDate = $date." 00:00:00";
                }

                if ((!isset($SQLDate)) || (time() < strtotime($date))) {
                    $date = false;
                    $validated = false;
                    $this->vars['error_code'] = "DATE";
                    $this->vars['error_string'] = "Invalid date (can't be in the future)";
                }
            }

            // Amount input validation
            if ($validated == true) { //Only do this if previous check passes
                $amount = (float) $amount;
                if (($amount <= 0) || $amount >= 1e12) {
                    $amount = false;
                    $validated = false;
                    $this->vars['error_code'] = "AMOUNT";
                    $this->vars['error_string'] = "Invalid Amount";
                }
            }

            // Asset input validation
            if ($validated == true) { //Only do this if previous check passes
                if (!$this->entityManager->getAsset($asset)) {
                    // ID provided doesnt exist.
                    $asset = false;
                    $validated = false;
                    $this->vars['error_code'] = "404";
                    $this->vars['error_string'] = "Asset not found";
                }
            }

            if ($validated == false) {
                $this->vars['error'] = true;
                $this->vars['data_date'] = $date;
                $this->vars['data_asset'] = $asset;
                $this->vars['data_amount'] = $amount;

                return false;
            } else {
                $this->vars['success'] = true;
                $this->vars['data_asset'] = $asset; //Pre-select last chosen asset (easier when adding lots)
                return array(':date' => $SQLDate, ':asset' => $asset, ':amount' => $amount);
            }
    }
}
