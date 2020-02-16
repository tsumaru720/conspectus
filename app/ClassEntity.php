<?php

class ClassEntity extends EntityObject {

    public function __construct(&$data) {
        $this->id = $data['id'];
        $this->description = $data['description'];
    }

}