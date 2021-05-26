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
            $vars['modifier'] = "=";
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
                $entity = $this->entityManager->getAsset($vars['item_id']);
                $this->pageTitle = "Breakdown - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
                $vars['asset_class'] = $entity->getClass();
                $vars['asset_class_id'] = $entity->getClassID();
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
                $entity = $this->entityManager->getClass($vars['item_id']);
                $this->pageTitle = "Breakdown - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
            }
        }


        while ($log = $this->db->fetch($logQuery)) {
            $vars['labels'][$log['yearMonth']] = $log['period'];

            $vars['log'][$log['asset_id']]['name'] = $log['description'];
            $vars['log'][$log['asset_id']]['data'][$log['yearMonth']] = $log['asset_value'];
            
            $last['log'][$log['asset_id']]['name'] = $log['description'];
            $last['log'][$log['asset_id']]['value'] = $log['asset_value'];
            $last['log'][$log['asset_id']]['yearMonth'] = $log['yearMonth'];
            $last['yearMonth'] = $log['yearMonth'];
        }

        if (isset($last)) {  // Probably no entries if this doesnt pass
            usort($last['log'], function($a, $b) {
                return $b['value'] - $a['value'];
            });
            // Used for Pie Charts
            $vars['mostRecent'] = $last['log'];

            // Remove assets that dont have a recent update (for pie charts)
            foreach ($vars['mostRecent'] as $key => $entry) {
                if ($entry['yearMonth'] != $last['yearMonth']) {
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
