import MySQLdb

conn = MySQLdb.connect(db='isubata',user='root')
c = conn.cursor()
c.execute('SELECT name,data FROM image')
for row in c.fetchall():
    f = open('icons/' + row[0],'wb')
    f.write(row[1])
    f.close()
conn.close()
