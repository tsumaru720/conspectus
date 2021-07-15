<?php

class PageLoader {

    private $theme = 'default';
    private $main = null;
    private $twig = null;
    private $vars = array();
    private $startedRender = false;
    
    private $displayHeader = true;
    private $displayFooter = true;

    public function __construct(&$main) {
        $this->main = $main;
        include $this->resolveTheme('theme.php');
    }

    public function hasStartedRender() {
        return $this->startedRender;
    }

    public function setTheme($theme) {
        $this->theme = $theme;
    }

    public function setVar($name, $value) {
        $this->vars[$name] = $value;
    }

    public function setFrame($header, $footer) {
        $this->displayHeader = $header;
        $this->displayFooter = $footer;
    }

    private function resolveTheme($location) {
        if (file_exists(__DIR__ . '/../themes/'.$this->theme.'/'.$location)) {
            return __DIR__ . '/../themes/'.$this->theme.'/'.$location;
        } else {
            return __DIR__ . '/../themes/default/'.$location;
        }
    }

    private function checkInterface($var) {
        if (!$var instanceof ThemeInterface) {
            //TODO theme this - but by this point we already have "Document" declared...
            echo 'Template page does not use ThemeInterface interface';
            die();
        }
    }

    public function display($pageName) {
        $this->startedRender = true;
        $output = "";
        // Add default path and custom theme path for template search
        $loader = new \Twig\Loader\FilesystemLoader(__DIR__.'/../themes/default/html');
        $loader->prependPath($this->resolveTheme('html'));

        // Also define a "default" namespace so we can always reference default
        // templates from within custom ones.
        $loader->addPath(__DIR__.'/../themes/default/html', 'default');

        $this->twig = new \Twig\Environment($loader);

        include $this->resolveTheme('app/'.$pageName.'.php');

        $doc = new Document($this->main, $this->twig, $this->vars);
        $this->checkInterface($doc);
        $this->setVar('page_title', $doc->getTitle());
        $scriptRegister = $doc->getRegister('script');
        $styleRegister = $doc->getRegister('style');

        if ($this->displayHeader) {
            include $this->resolveTheme('app/__header.php');
            $header = new Header($this->main, $this->twig, $this->vars);
            $this->checkInterface($header);
            $header->setRegister('style', $styleRegister);
            $header->render();
            $output .= $header->getRendered();
            if ($this->main->getDb()->getError()) {
                $this->main->fatalErr("500", "Error loading header: ". $this->main->getDb()->getError()['message']);
            }
        }

        $doc->render();
        $output .= $doc->getRendered();
        if ($this->main->getDb()->getError()) {
            $this->main->fatalErr("500", "Error loading document: ". $this->main->getDb()->getError()['message']);
        }

        if ($this->displayFooter) {
            include $this->resolveTheme('app/__footer.php');
            $footer = new Footer($this->main, $this->twig, $this->vars);
            $this->checkInterface($footer);
            $footer->setRegister('script', $scriptRegister);
            $footer->render();
            $output .= $footer->getRendered();
            if ($this->main->getDb()->getError()) {
                $this->main->fatalErr("500", "Error loading header: ". $this->main->getDb()->getError()['message']);
            }
        }

        echo $output;
    }

}
