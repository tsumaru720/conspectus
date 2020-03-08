<?php

class Document extends Theme {

    protected $pageTitle = 'Class Manager';
    private $db = null;
    private $twig = null;

    public function __construct(&$main, &$twig, $vars) {

        $this->twig = $twig;
        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();
        $this->vars = $vars;

        if ($vars['action'] == "new") {
            $this->pageTitle .= " - New Class";
            $this->vars['action_string'] = "Add new class";
        } elseif ($vars['action'] == "edit") {
            $this->pageTitle .= " - Edit Class";
            $this->vars['action_string'] = "Edit class";
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
            $this->document = $this->twig->load('class_manager.html');
        } elseif ($action == "edit") {
            $class = $this->entityManager->getClass($this->vars['item_id']);
            $this->vars['form_description'] = $class->getDescription();
            $this->document = $this->twig->load('class_manager.html');
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
            if ($data = $this->classValidation()) {
                $this->db->query("INSERT INTO `asset_classes` (`id`, `description`) VALUES (NULL, :description)", $data);
            }
            $this->document = $this->twig->load('class_manager.html');
        } elseif ($action == "edit") {
            if ($data = $this->classValidation()) {
                $data['class_id'] = $this->vars['item_id'];
                $this->db->query("UPDATE `asset_classes` SET `description` = :description WHERE `asset_classes`.`id` = :class_id", $data);
                header('Location: /view/class/'.$data['class_id']);
            } else {
                $this->document = $this->twig->load('class_manager.html');
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

    private function classValidation() {
            $description = $this->validateInput('description');
            $validated = true;

            if ($description == false) {
                $validated = false;
                $this->vars['error_code'] = "EMPTY";
                $this->vars['error_string'] = "Input is empty";
            }

            if (strlen($description) > 20) {
                // 40 is the size set our DB schema
                $validated = false;
                $this->vars['error_code'] = "STRLEN";
                $this->vars['error_string'] = "Class name is too long - Max: 20";
            } else {
                $data = array(':description' => $description);
                $q = $this->db->query("SELECT id from asset_classes WHERE description = :description", $data);
                if ($q->rowCount() > 0) {
                    $my_id = 0;
                    $dupe = $this->db->fetch($q);
                    if (array_key_exists('item_id', $this->vars)) {
                        $my_id = $this->vars['item_id'];
                    }

                    if ($dupe['id'] != $my_id) {
                        // Name specified already exists - its an exact match
                        // Probably dont want duplicates.
                        $validated = false;
                        $this->vars['error_code'] = "DUPLICATE";
                        $this->vars['error_string'] = "Class with that name already exists";
                    }
                }
            }

            if ($validated == false) {
                $this->vars['error'] = true;
                $this->vars['form_description'] = $description;

                return false;
            } else {
                $this->vars['success'] = true;
                return array(':description' => $description);
            }
    }
}
