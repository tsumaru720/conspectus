<?php

class Document extends Theme {

    protected $pageTitle = 'Asset Manager';
    private $db = null;

    public function __construct(&$main, &$twig, $vars) {

        $this->db = $main->getDB();
        $this->vars = $vars;

        $q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
        while ($item = $this->db->fetch($q)) {
            $this->vars['class_menu_items'][] = $item;
        }

        // Other requests types should never get this far
        // due to bramus router matching.
        if ($_SERVER['REQUEST_METHOD'] == "GET") {
            $this->processGet($twig, $vars['action']);
        } elseif ($_SERVER['REQUEST_METHOD'] == "POST") {
            $this->processPost($twig, $vars['action']);
        }
        
    }

    private function processGet(&$twig, $action) {
        if ($action == "new") {
            $this->pageTitle .= " - New Asset";
            $vars['page_title'] = $this->pageTitle;
            $this->document = $twig->load('asset_manager_new.html');
        } elseif ($action == "edit") {
            echo "not implemented yet";
            die();
        } elseif ($action == "delete") {
            echo "not implemented yet";
            die();
        } else {
            // Should probably handle this nicer
            // Chances are someone's trying to break something
            // So meh.
            die();
        }
    }

    private function processPost(&$twig, $action) {
        if ($action == "new") {
            $description = $this->validateInput('description');
            $class = $this->validateInput('class');

            $validated = true;
            if (($description == false) || $class == false) {
                $validated = false;
                $this->vars['error_code'] = "EMPTY";
                $this->vars['error_string'] = "Input is empty";
            }

            if (strlen($description) > 40) {
                // 40 is the size set our DB schema
                $validated = false;
                $this->vars['error_code'] = "STRLEN";
                $this->vars['error_string'] = "Asset name is too long - Max: 40";
            } else {
                $data = array(':description' => $description);
                $q = $this->db->query("SELECT id from asset_list WHERE description = :description", $data);
                if ($q->rowCount() > 0) {
                    // Name specified already exists - its an exact match
                    // Probably dont want duplicates.
                    $validated = false;
                    $this->vars['error_code'] = "DUPLICATE";
                    $this->vars['error_string'] = "Asset with that name already exists";
                }
            }

            $data = array(':class_id' => $class);
            $q = $this->db->query("SELECT id from asset_classes WHERE id = :class_id", $data);
            if ($q->rowCount() == 0) {
                // ID provided doesnt exist.
                $class = false;
                $validated = false;
                $this->vars['error_code'] = "404";
                $this->vars['error_string'] = "Class not found";
            }
            
            if ($validated == false) {
                $this->vars['error'] = true;
                $this->vars['form_description'] = $description;
                $this->vars['form_class'] = $class;
            } else {
                $this->vars['success'] = true;
                $this->vars['form_class'] = $class; //Pre-select last chosen class (easier when adding lots)
                $data = array(':description' => $description, ':class_id' => $class);
                $q = $this->db->query("INSERT INTO `asset_list` (`id`, `asset_class`, `description`) VALUES (NULL, :class_id, :description)", $data);
            }

            $this->document = $twig->load('asset_manager_new.html');
        } elseif ($action == "edit") {
            echo "not implemented yet";
            die();
        } elseif ($action == "delete") {
            echo "not implemented yet";
            die();
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

}
