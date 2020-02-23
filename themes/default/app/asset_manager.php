<?php

class Document extends Theme {

    protected $pageTitle = 'Asset Manager';
    private $db = null;
    private $twig = null;

    public function __construct(&$main, &$twig, $vars) {

        $this->twig = $twig;
        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();
        $this->vars = $vars;

        $q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
        while ($item = $this->db->fetch($q)) {
            $this->vars['class_menu_items'][] = $item;
        }

        if ($vars['action'] == "new") {
            $this->pageTitle .= " - New Asset";
            $this->vars['action_string'] = "Add new asset";
        } elseif ($vars['action'] == "edit") {
            $this->pageTitle .= " - Edit Asset";
            $this->vars['action_string'] = "Edit asset";
        }

        // Other requests types should never get this far
        // due to bramus router matching.
        if ($_SERVER['REQUEST_METHOD'] == "GET") {
            $this->processGet($vars['action']);
        } elseif ($_SERVER['REQUEST_METHOD'] == "POST") {
            $this->processPost($vars['action']);
        }
    }

    private function processGet($action) {
        if ($action == "new") {
            $this->document = $this->twig->load('asset_manager.html');
        } elseif ($action == "edit") {
            if (!$asset = $this->entityManager->getAsset($this->vars['item_id'])) {
                echo "Invalid Asset";
                die();
                //TODO make this error nicer
            }
            $this->vars['form_description'] = $asset->getDescription();
            $this->vars['form_class'] = $asset->getClassID();

            $this->document = $this->twig->load('asset_manager.html');
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

    private function processPost($action) {
        if ($action == "new") {
            if ($data = $this->assetValidation()) {
                $this->db->query("INSERT INTO `asset_list` (`id`, `asset_class`, `description`) VALUES (NULL, :class_id, :description)", $data);
            }
            $this->document = $this->twig->load('asset_manager.html');
        } elseif ($action == "edit") {
            if ($this->entityManager->getAsset($this->vars['item_id'])) {
                echo "Invalid Asset";
                die();
                //TODO make this error nicer
            }
            if ($data = $this->assetValidation(true)) {
                $data['asset_id'] = $this->vars['item_id'];
                $this->db->query("UPDATE `asset_list` SET `asset_class` = :class_id, `description` = :description WHERE `asset_list`.`id` = :asset_id", $data);
                header('Location: /view/asset/'.$data['asset_id']);
            } else {
                $this->document = $this->twig->load('asset_manager.html');
            }
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

    private function assetValidation($allowDuplicate = false) {
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
                if ($allowDuplicate == false) {
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
            }

            if (!$this->entityManager->getClass($class)) {
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

                return false;
            } else {
                $this->vars['success'] = true;
                $this->vars['form_class'] = $class; //Pre-select last chosen class (easier when adding lots)

                return array(':description' => $description, ':class_id' => $class);
            }
    }
}
