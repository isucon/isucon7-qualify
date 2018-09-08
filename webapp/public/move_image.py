import MySQLdb

# conn = MySQLdb.connect(db='isubata',user='root',passwd='password',charset='utf8mb4')
conn = MySQLdb.connect(db='isubata',user='root')
c = conn.cursor()
#tableが既にある場合は一回削除します
c.execute('SELECT name,data FROM image')
for row in c.fetchall():
    f = open('icons/' + row[0],'wb')
    f.write(row[1])
    f.close()
conn.close()
