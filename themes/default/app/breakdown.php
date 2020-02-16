<?php

class Document extends Theme {

    protected $pageTitle = 'Breakdown';
    private $db = null;
    private $entityManager = null;

    public function __construct(&$main, &$twig, $vars) {

        $vars['page_title'] = $this->pageTitle;

        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();

        if ($vars['left_menu'] != 'all') {
            if (!is_numeric($vars['item_id']) || $vars['item_id'] < 0) {
                echo "bad id";
                die();
                //TODO make this error nicer
            } else {
                $vars['modifier'] = "=";
            }
        } else {
            $vars['item_id'] = 0;
            $vars['modifier'] = ">";
        }
        $data = array(':item_id' => $vars['item_id']);

        if ($vars['type'] == 'asset') {
            $logQuery = $this->db->query("SELECT
                                            asset_id,
                                            description,
                                            deposit_value,
                                            asset_value,
                                            (asset_value - deposit_value) AS gain,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        ORDER BY
                                            yearMonth ASC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $asset = $this->entityManager->getAsset($vars['item_id']);
                if ($asset) {
                    $this->pageTitle = "Breakdown - ".$asset->getDescription();
                    $vars['page_title'] = $asset->getDescription();
                    $vars['asset_class'] = $asset->getClass();
                } else {
                    echo "invalid asset";
                    die();
                    //TODO make this error nicer
                }
            }
        } elseif ($vars['type'] == 'class') {
            $logQuery = $this->db->query("SELECT
                                            asset_id,
                                            asset_list.description,
                                            asset_value,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH
                                        FROM
                                            epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id = :item_id
                                        ORDER BY
                                            yearMonth ASC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $class = $this->entityManager->getClass($vars['item_id']);
                if ($class) {
                    $this->pageTitle = "Breakdown - ".$class->getDescription();
                    $vars['page_title'] = $class->getDescription();
                } else {
                    echo "invalid class";
                    die();
                    //TODO make this error nicer
                }
            }
        } else {
            echo "invalid type";
            die();
            //TODO make this error nicer
        }


        while ($log = $this->db->fetch($logQuery)) {
            $vars['labels'][$log['yearMonth']] = $log['period'];

            $vars['log'][$log['asset_id']]['name'] = $log['description'];
            $vars['log'][$log['asset_id']]['data'][$log['yearMonth']] = $log['asset_value'];
            
            $last['log'][$log['asset_id']]['name'] = $log['description'];
            $last['log'][$log['asset_id']]['value'] = $log['asset_value'];
            $last['yearMonth'] = $log['yearMonth'];
        }

        if (isset($last)) {  // Probably no entries if this doesnt pass
            usort($last['log'], function($a, $b) {
                return $b['value'] - $a['value'];
            });
            // Used for Pie Charts
            $vars['mostRecent'] = $last['log'];
            // Remove zero values from recent data
            foreach ($vars['mostRecent'] as $key => $entry) {
                if ($entry['value'] <= 0) {
                    unset($vars['mostRecent'][$key]);
                }
            }

            foreach ($vars['log'] as $key => $entry) {
                foreach ($vars['labels'] as $period => $v) {
                    if (!array_key_exists($period, $entry['data'])) {
                        $vars['log'][$key]['data'][$period] = 0;
                        ksort($vars['log'][$key]['data']);
                    }
                }
            }

            if ($vars['left_menu'] == 'all') {
                $data = array(':yearMonth' => $last['yearMonth']);
                $logQuery = $this->db->query("SELECT
                                                asset_classes.id,
                                                asset_classes.description as description,
                                                sum(asset_value) as asset_value,
                                                DATE_FORMAT(epoch, '%b %Y') AS period,
                                                EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                            FROM
                                                asset_log
                                            LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                            LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                            WHERE
                                                EXTRACT(YEAR_MONTH FROM epoch) = :yearMonth
                                            GROUP BY
                                                period,
                                                yearMonth,
                                                asset_classes.id,
                                                asset_classes.description
                                            ORDER BY
                                                yearMonth ASC,
                                                description ASC", $data);
                while ($log = $this->db->fetch($logQuery)) {
                    $vars['class_mostRecent'][] = array('name' => $log['description'], 'value' => $log['asset_value']);
                }
                usort($vars['class_mostRecent'], function($a, $b) {
                    return $b['value'] - $a['value'];
                });
            }
        }

        //$this->setRegister('script', "https://cdnjs.cloudflare.com/ajax/libs/Chart.js/2.7.3/Chart.min.js");
        $this->vars = $vars;
        $this->document = $twig->load('breakdown.html');
    }

}
