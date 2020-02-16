<?php

abstract class EntityObject {
    protected $id = null;
    protected $description = null;

    public function getID() {
        return $this->id;
    }

    public function getDescription() {
        return $this->description;
    }

}