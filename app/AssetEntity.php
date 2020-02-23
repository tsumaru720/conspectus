<?php

class AssetEntity extends EntityObject {
    private $class_id = null;
    private $class = null;


    public function __construct(&$data) {
        $this->id = $data['id'];
        $this->description = $data['description'];
        $this->class_id = $data['class_id'];
        $this->class = $data['class'];
    }

    public function getClass() {
        return $this->class;
    }

    public function getClassID() {
        return $this->class_id;
    }
}