<?php

class Document extends Theme {

    protected $pageTitle = 'Analytics';
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
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            if ($vars['item_id'] > 0) {
                $asset = $this->entityManager->getAsset($vars['item_id']);
                $object = $asset;

                if ($asset) {
                    $this->pageTitle = "Analytics - ".$asset->getDescription();
                    $vars['page_title'] = $asset->getDescription();
                    $vars['asset_class'] = $asset->getClass();
                } else {
                    echo "invalid asset";
                    die();
                    //TODO make this error nicer
                }
            }
        } elseif ($vars['type'] == 'class') {
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            if ($vars['item_id'] > 0) {
                $class = $this->entityManager->getClass($vars['item_id']);
                if ($class) {
                    $this->pageTitle = "Analytics - ".$class->getDescription();
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
            //TODO Tidy up this whole mess of code - a lot of it can be put in generic functions
        while ($period = $this->db->fetch($dataQuery)) {
            $period['value_delta'] = 0;
            $period['growth_factor'] = 0;

            if (!isset($vars['periodData'][$period['year']])) {
                if ($period['year'] == date("Y")) {
                    $vars['periodData'][$period['year']]['label'] = "YTD";
                } else {
                    $vars['periodData'][$period['year']]['label'] = $period['year'];
                }
                $vars['periodData'][$period['year']]['twr'] = 0;
                $vars['periodData'][$period['year']]['start'] = $period['asset_total'];

            }

            if (isset($last)) {
                $period['value_delta'] = number_format($period['deposit_total'] - $last['deposit_total'], 2, '.', '');

                if (($period['asset_total'] != 0) && ($last['asset_total'] != 0)) {
                    $period['growth_factor'] = $period['asset_total'] / ($last['asset_total'] + $period['value_delta']);
                }
                if ($vars['periodData'][$period['year']]['twr'] == 0) {
                    if ($period['year'] == $last['year']) {
                        $vars['periodData'][$period['year']]['twr'] = $period['growth_factor'];
                    } else {
                        // Add TWR factor to previous year
                        if ($vars['periodData'][$last['year']]['twr'] == 0) {
                            $vars['periodData'][$last['year']]['twr'] = $period['growth_factor'];
                        } else {
                            $vars['periodData'][$last['year']]['twr'] = $vars['periodData'][$last['year']]['twr'] * $period['growth_factor'];
                        }

                        // Recalculate value percentage increase for the year
                        $vars['periodData'][$last['year']]['end'] = $period['asset_total'];
                        $start = $vars['periodData'][$last['year']]['start'];
                        $end = $vars['periodData'][$last['year']]['end'];
                        $vars['periodData'][$last['year']]['increase'] = number_format((($end / $start) - 1) * 100, 2, '.', '');
                    }
                } else {
                    $vars['periodData'][$period['year']]['twr'] = $vars['periodData'][$period['year']]['twr'] * $period['growth_factor'];
                }
            }

            $vars['periodData'][$period['year']]['end'] = $period['asset_total'];

            $start = $vars['periodData'][$period['year']]['start'];
            $end = $vars['periodData'][$period['year']]['end'];

            if ($start != 0) {
                $vars['periodData'][$period['year']]['increase'] = number_format((($end / $start) - 1) * 100, 2, '.', '');
            } else {
                $vars['periodData'][$period['year']]['start'] = $period['asset_total'];
                $vars['periodData'][$period['year']]['increase'] = 0;
            }

            $last = $period;
        }

        if (array_key_exists('periodData', $vars)) {
            foreach ($vars['periodData'] as $key => $value) {
                if ($value['twr'] != 0) {
                    $value['twr'] = ($value['twr'] - 1) * 100;
                    $vars['periodData'][$key]['twr'] = number_format($value['twr'],2);
                }
            }
        }

        $this->vars = $vars;
        $this->document = $twig->load('analytics.html');
    }

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
